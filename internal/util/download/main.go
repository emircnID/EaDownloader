package download

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"eadownloader/internal/models"
	"eadownloader/internal/networking"
	"eadownloader/internal/util/download/chunked"
	"eadownloader/internal/util/download/retry"
	"eadownloader/internal/util/download/segmented"
	"eadownloader/internal/util/libav"
	"github.com/google/uuid"
)

var ytDLPProgressPattern = regexp.MustCompile(`\b(\d+(?:\.\d+)?)%`)

func DownloadFile(
	ctx *models.ExtractorContext,
	urlList []string,
	fileName string,
	settings *models.DownloadSettings,
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("nil extractor context")
	}
	settings = ensureDownloadSettings(settings)
	ensureDownloadDir()

	client := ctx.HTTPClient.AsDownloadClient()

	filePath := ToPath(fileName)
	ctx.FilesTracker.Add(filePath)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lastErr error
	for _, url := range urlList {
		ctx.Debugf("attempting download from: %s", url)
		if err := resetFile(file); err != nil {
			return "", err
		}

		cd, err := chunked.New(ctx.Context, client, url, settings)
		if err != nil {
			err = downloadSequential(ctx, client, url, file, settings)
			if err != nil {
				lastErr = err
				continue
			}
		} else {
			err = cd.Download(ctx, file, settings.NumConnections)
			if err != nil {
				lastErr = err
				continue
			}
		}

		if !settings.SkipRemux {
			outputPath := strings.TrimSuffix(
				filePath,
				filepath.Ext(filePath),
			) + "_remuxed" + filepath.Ext(filePath)
			ctx.FilesTracker.Add(outputPath)

			err = libav.RemuxFile(filePath, outputPath)
			if err != nil {
				ctx.Warnf("remuxing failed, using original file: %v", err)
				return filePath, nil
			}

			// replace original file with remuxed file
			os.Rename(outputPath, filePath)
		}

		return filePath, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no URLs to download")
	}
	return "", lastErr
}

func DownloadFileWithYtDLP(
	ctx *models.ExtractorContext,
	fileName string,
	settings *models.DownloadSettings,
) (string, error) {
	settings = ensureDownloadSettings(settings)
	ensureDownloadDir()

	filePath := ToPath(fileName)
	ctx.FilesTracker.Add(filePath)

	args := ytDLPDownloadArgs(filePath, settings)
	ctx.Debugf("attempting yt-dlp download with format: %s", settings.YtDLPFormat)

	cmd := exec.CommandContext(ctx.Context, "yt-dlp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	lastProgress := -1
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		progress := ytDLPProgressBucket(scanner.Text())
		if progress < 0 || progress == lastProgress {
			continue
		}
		lastProgress = progress
		ctx.Progress(fmt.Sprintf("YouTube indiriliyor... %d%%", progress))
	}
	if err := scanner.Err(); err != nil {
		ctx.Warnf("failed to read yt-dlp progress: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("yt-dlp download failed: %w; stderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	resolvedPath, err := resolveYtDLPOutputPath(filePath)
	if err != nil {
		return "", err
	}
	if resolvedPath != filePath {
		ctx.FilesTracker.Add(resolvedPath)
	}

	return resolvedPath, nil
}

func ytDLPDownloadArgs(filePath string, settings *models.DownloadSettings) []string {
	concurrentFragments := fmt.Sprintf("%d", max(settings.NumConnections, 1))
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--force-ipv4",
		"--socket-timeout", "15",
		"--retries", fmt.Sprintf("%d", max(settings.Retries, 1)),
		"--fragment-retries", fmt.Sprintf("%d", max(settings.Retries, 1)),
		"--concurrent-fragments", concurrentFragments,
		"--http-chunk-size", "10M",
		"--newline",
		"-f", settings.YtDLPFormat,
		"-o", filePath,
	}
	if settings.YtDLPSort != "" {
		args = append(args, "--format-sort", settings.YtDLPSort)
	}
	if settings.YtDLPCookieJar != "" {
		args = append(args, "--cookies", settings.YtDLPCookieJar)
	}
	if settings.YtDLPArgs != "" {
		args = append(args, "--extractor-args", settings.YtDLPArgs)
	}
	if settings.YtDLPAudio {
		args = append(args, "-x", "--audio-format", "mp3", "--audio-quality", "0")
	} else {
		args = append(args, "--merge-output-format", "mp4")
	}
	return append(args, settings.YtDLPURL)
}

func ytDLPProgressBucket(line string) int {
	matches := ytDLPProgressPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return -1
	}

	percent, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return -1
	}
	if percent >= 100 {
		return 100
	}
	if percent < 0 {
		return -1
	}
	return int(percent/10) * 10
}

func resolveYtDLPOutputPath(filePath string) (string, error) {
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	extension := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, extension)
	matches, err := filepath.Glob(base + ".*")
	if err != nil {
		return "", err
	}
	for _, match := range matches {
		if strings.HasSuffix(match, ".part") ||
			strings.HasSuffix(match, ".ytdl") ||
			strings.HasSuffix(match, ".temp") {
			continue
		}
		return match, nil
	}

	return "", fmt.Errorf("yt-dlp output file not found: %s", filePath)
}

func DownloadFileWithSegments(
	ctx *models.ExtractorContext,
	initSegmentURL string,
	segmentURLs []string,
	fileName string,
	settings *models.DownloadSettings,
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("nil extractor context")
	}
	settings = ensureDownloadSettings(settings)
	ensureDownloadDir()

	client := ctx.HTTPClient.AsDownloadClient()

	filePath := ToPath(fileName)
	ctx.FilesTracker.Add(filePath)

	tempDir := ToPath("segments" + uuid.NewString()[:8])
	ctx.FilesTracker.Add(tempDir)

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	ctx.Debugf("attempting download from: %s", segmentURLs[0])

	sd := segmented.New(
		ctx.Context, client,
		tempDir, segmentURLs,
		&segmented.SegmentedDownloaderOptions{
			InitSegment:      initSegmentURL,
			DownloadSettings: settings,
		},
	)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	err = sd.Download(ctx.Context, file, settings.NumConnections)
	if err != nil {
		return "", err
	}

	if !settings.SkipRemux {
		outputPath := strings.TrimSuffix(
			filePath,
			filepath.Ext(filePath),
		) + "_remuxed" + filepath.Ext(filePath)
		ctx.FilesTracker.Add(outputPath)

		err = libav.RemuxFile(filePath, outputPath)
		if err != nil {
			ctx.Warnf("remuxing failed, using original file: %v", err)
			return filePath, nil
		}

		// replace original file with remuxed file
		os.Rename(outputPath, filePath)
	}

	return filePath, nil
}

func DownloadFileInMemory(
	ctx *models.ExtractorContext,
	urlList []string,
	settings *models.DownloadSettings,
) (*bytes.Reader, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil extractor context")
	}
	settings = ensureDownloadSettings(settings)

	client := ctx.HTTPClient.AsDownloadClient()
	maxRetries := max(settings.Retries, 1)
	var lastErr error

	for _, url := range urlList {
		for attempt := range maxRetries {
			ctx.Debugf("attempting download from: %s (attempt %d/%d)", url, attempt+1, maxRetries)
			resp, err := client.FetchWithContext(
				ctx.Context,
				http.MethodGet,
				url, &networking.RequestParams{
					Headers: settings.Headers,
					Cookies: settings.Cookies,
				},
			)
			if err != nil {
				lastErr = err
				if waitErr := retry.Sleep(ctx.Context, attempt, nil); waitErr != nil {
					return nil, waitErr
				}
				continue
			}

			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				headers := resp.Header
				resp.Body.Close()
				if retry.IsStatus(resp.StatusCode) {
					if waitErr := retry.Sleep(ctx.Context, attempt, headers); waitErr != nil {
						return nil, waitErr
					}
					continue
				}
				break
			}

			if resp.ContentLength > 0 && resp.ContentLength > maxInMemoryDownloadSize {
				lastErr = fmt.Errorf("file too large to download in memory: %d bytes", resp.ContentLength)
				resp.Body.Close()
				continue
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
				lastErr = err
				if waitErr := retry.Sleep(ctx.Context, attempt, nil); waitErr != nil {
					return nil, waitErr
				}
				continue
			}

			resp.Body.Close()
			return bytes.NewReader(data), nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no URLs to download")
	}
	return nil, fmt.Errorf("all download attempts failed: %w", lastErr)
}

func downloadSequential(
	ctx *models.ExtractorContext,
	client *networking.HTTPClient,
	url string,
	file *os.File,
	settings *models.DownloadSettings,
) error {
	settings = ensureDownloadSettings(settings)
	maxRetries := max(settings.Retries, 1)
	var lastErr error

	for attempt := range maxRetries {
		if err := resetFile(file); err != nil {
			return err
		}

		resp, err := client.FetchWithContext(
			ctx.Context,
			http.MethodGet, url,
			&networking.RequestParams{
				Headers: settings.Headers,
				Cookies: settings.Cookies,
			},
		)
		if err != nil {
			lastErr = err
			if waitErr := retry.Sleep(ctx.Context, attempt, nil); waitErr != nil {
				return waitErr
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			headers := resp.Header
			resp.Body.Close()
			if retry.IsStatus(resp.StatusCode) {
				if waitErr := retry.Sleep(ctx.Context, attempt, headers); waitErr != nil {
					return waitErr
				}
				continue
			}
			return lastErr
		}

		_, err = io.Copy(file, resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if waitErr := retry.Sleep(ctx.Context, attempt, nil); waitErr != nil {
				return waitErr
			}
			continue
		}

		return nil
	}

	return lastErr
}

func resetFile(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err := file.Seek(0, io.SeekStart)
	return err
}
