package localization

import "github.com/nicksnyder/go-i18n/v2/i18n"

var (
	Language = &i18n.Message{
		ID:    "Language",
		Other: "English",
	}
	StartMessage = &i18n.Message{
		ID:    "StartMessage",
		Other: "Welcome {{.Name}}. Send a supported social media link, and I will fetch the media for you.",
	}
	AddButton = &i18n.Message{
		ID:    "AddButton",
		Other: "Add to a Group",
	}
	ErrorMessage = &i18n.Message{
		ID:    "ErrorMessage",
		Other: "An error occurred. Please try again later.",
	}
	AddedToGroupMessage = &i18n.Message{
		ID:    "AddedToGroupMessage",
		Other: "Thank you for adding me! Use the /settings command to configure the bot for this group.",
	}
	SettingsButton = &i18n.Message{
		ID:    "SettingsButton",
		Other: "Settings",
	}
	LanguageButton = &i18n.Message{
		ID:    "LanguageButton",
		Other: "Language",
	}
	PrivateSettingsMessage = &i18n.Message{
		ID:    "PrivateSettingsMessage",
		Other: "Use the buttons below to change your personal bot settings.",
	}
	GroupSettingsMessage = &i18n.Message{
		ID:    "GroupSettingsMessage",
		Other: "Use the buttons below to change this group's bot settings.",
	}
	BackButton = &i18n.Message{
		ID:    "BackButton",
		Other: "Back",
	}
	SelectLanguageMessage = &i18n.Message{
		ID:    "SelectLanguageMessage",
		Other: "Select your preferred language.",
	}
	CaptionsSettingsMessage = &i18n.Message{
		ID:    "CaptionsSettingsMessage",
		Other: "When enabled, adds the original description to downloaded content if available.",
	}
	NsfwSettingsMessage = &i18n.Message{
		ID:    "NsfwSettingsMessage",
		Other: "When enabled, allows NSFW content to be downloaded in this chat.\n\nWarning: This type of content may violate Telegram's Terms of Service and result in group restrictions.",
	}
	SilentModeSettingsMessage = &i18n.Message{
		ID:    "SilentModeSettingsMessage",
		Other: "When enabled, the bot will not send error messages.",
	}
	MediaAlbumSettingsMessage = &i18n.Message{
		ID:    "MediaAlbumSettingsMessage",
		Other: "Select the maximum number of files allowed in a single media album.",
	}
	InlineLoadingMessage = &i18n.Message{
		ID:    "InlineLoadingMessage",
		Other: "Loading... Please wait.",
	}
	InlineProcessingMessage = &i18n.Message{
		ID:    "InlineProcessingMessage",
		Other: "Media shared! Processing the download... Please wait.",
	}
	InlineShareMessage = &i18n.Message{
		ID:    "InlineShareMessage",
		Other: "Share this media",
	}
	NoPermission = &i18n.Message{
		ID:    "NoPermission",
		Other: "You do not have permission to perform this action.",
	}
	CloseButton = &i18n.Message{
		ID:    "CloseButton",
		Other: "Close",
	}
	MediaAlbumButton = &i18n.Message{
		ID:    "MediaAlbumButton",
		Other: "Media Album",
	}
	SilentModeButton = &i18n.Message{
		ID:    "SilentModeButton",
		Other: "Silent Mode",
	}
	CaptionsButton = &i18n.Message{
		ID:    "CaptionsButton",
		Other: "Captions",
	}
	NsfwButton = &i18n.Message{
		ID:    "NsfwButton",
		Other: "NSFW",
	}
	ExtractorsButton = &i18n.Message{
		ID:    "ExtractorsButton",
		Other: "Platforms",
	}
	DisabledExtractorsSettingsMessage = &i18n.Message{
		ID:    "DisabledExtractorsSettingsMessage",
		Other: "Select which platforms should be disabled. Links from disabled platforms will be ignored by the bot.",
	}
	DeleteLinksButton = &i18n.Message{
		ID:    "DeleteProcessedButton",
		Other: "Links",
	}
	DeleteLinksSettingsMessage = &i18n.Message{
		ID:    "DeleteProcessedSettingsMessage",
		Other: "When enabled, deletes the user's original message after the link is processed successfully.",
	}
	SupportedExtractorsMessage = &i18n.Message{
		ID:    "SupportedExtractorsMessage",
		Other: "Supported platforms",
	}
	EnabledButton = &i18n.Message{
		ID:    "EnabledButton",
		Other: "Enabled",
	}
	DisabledButton = &i18n.Message{
		ID:    "DisabledButton",
		Other: "Disabled",
	}
	ErrorUnavailable = &i18n.Message{
		ID:    "ErrorUnavailable",
		Other: "This content is unavailable.",
	}
	ErrorTimeout = &i18n.Message{
		ID:    "ErrorTimeout",
		Other: "The download timed out. Please try again later.",
	}
	ErrorUnsupportedImageFormat = &i18n.Message{
		ID:    "ErrorUnsupportedImageFormat",
		Other: "Unsupported image format.",
	}
	ErrorUnsupportedExtractorType = &i18n.Message{
		ID:    "ErrorUnsupportedExtractorType",
		Other: "Unsupported platform type.",
	}
	ErrorMediaAlbumLimitExceeded = &i18n.Message{
		ID:    "ErrorMediaAlbumLimitExceeded",
		Other: "The media album limit exceeds the maximum allowed for this group. Use /settings to increase the limit.",
	}
	ErrorMediaAlbumGlobalLimitExceeded = &i18n.Message{
		ID:    "ErrorMediaAlbumGlobalLimitExceeded",
		Other: "The media album limit exceeds the maximum allowed for this instance.",
	}
	ErrorGeoRestrictedContent = &i18n.Message{
		ID:    "ErrorGeoRestrictedContent",
		Other: "This content has geo-restrictions and cannot be accessed from the server's location.",
	}
	ErrorNSFWNotAllowed = &i18n.Message{
		ID:    "ErrorNSFWNotAllowed",
		Other: "This content is marked as NSFW and cannot be downloaded in this group. Use /settings to allow NSFW content or use the bot privately.",
	}
	ErrorInlineMediaAlbum = &i18n.Message{
		ID:    "ErrorInlineMediaAlbum",
		Other: "Media albums cannot be downloaded in inline mode. Use the bot in a group or private chat.",
	}
	ErrorAuthenticationNeeded = &i18n.Message{
		ID:    "ErrorAuthenticationNeeded",
		Other: "This instance is not authenticated with this service.",
	}
	ErrorFileTooLarge = &i18n.Message{
		ID:    "ErrorFileTooLarge",
		Other: "This file is too large and exceeds the maximum allowed size for this instance.",
	}
	ErrorTelegramFileTooLarge = &i18n.Message{
		ID:    "ErrorTelegramFileTooLarge",
		Other: "This file is too large for Telegram.",
	}
	ErrorDurationTooLong = &i18n.Message{
		ID:    "ErrorDurationTooLong",
		Other: "This video is too long and exceeds the maximum allowed duration for this instance.",
	}
	ErrorPaidContent = &i18n.Message{
		ID:    "ErrorPaidContent",
		Other: "This content is paid and requires a subscription to access.",
	}
	ErrorAgeRestricted = &i18n.Message{
		ID:    "ErrorAgeRestricted",
		Other: "This content is age-restricted and cannot be accessed.",
	}
	ErrorPermissionDenied = &i18n.Message{
		ID:    "ErrorPermissionDenied",
		Other: "The bot does not have sufficient permissions to send this media. Please grant the necessary permissions and try again.",
	}
	AdminTitle = &i18n.Message{
		ID:    "AdminTitle",
		Other: "Admin Panel",
	}
	AdminOperationPanel = &i18n.Message{
		ID:    "AdminOperationPanel",
		Other: "Operations panel",
	}
	AdminGeneralStatus = &i18n.Message{
		ID:    "AdminGeneralStatus",
		Other: "General Status",
	}
	AdminChooseSection = &i18n.Message{
		ID:    "AdminChooseSection",
		Other: "Choose a section.",
	}
	AdminUsers = &i18n.Message{
		ID:    "AdminUsers",
		Other: "Users",
	}
	AdminGroups = &i18n.Message{
		ID:    "AdminGroups",
		Other: "Groups",
	}
	AdminDownloads = &i18n.Message{
		ID:    "AdminDownloads",
		Other: "Downloads",
	}
	AdminMuted = &i18n.Message{
		ID:    "AdminMuted",
		Other: "Muted",
	}
	AdminBanned = &i18n.Message{
		ID:    "AdminBanned",
		Other: "Banned",
	}
	AdminAnalytics = &i18n.Message{
		ID:    "AdminAnalytics",
		Other: "Analytics",
	}
	AdminSystemPanel = &i18n.Message{
		ID:    "AdminSystemPanel",
		Other: "System Panel",
	}
	AdminHomeButton = &i18n.Message{
		ID:    "AdminHomeButton",
		Other: "Home",
	}
	AdminTotal = &i18n.Message{
		ID:    "AdminTotal",
		Other: "Total",
	}
	AdminPage = &i18n.Message{
		ID:    "AdminPage",
		Other: "Page",
	}
	AdminPeriodLabel = &i18n.Message{
		ID:    "AdminPeriodLabel",
		Other: "Period",
	}
	AdminPeriod24h = &i18n.Message{
		ID:    "AdminPeriod24h",
		Other: "24 hours",
	}
	AdminPeriod7d = &i18n.Message{
		ID:    "AdminPeriod7d",
		Other: "7 days",
	}
	AdminPeriod30d = &i18n.Message{
		ID:    "AdminPeriod30d",
		Other: "30 days",
	}
	AdminPeriodAll = &i18n.Message{
		ID:    "AdminPeriodAll",
		Other: "all time",
	}
	AdminUptimeLabel = &i18n.Message{
		ID:    "AdminUptimeLabel",
		Other: "Uptime",
	}
	AdminGoroutineLabel = &i18n.Message{
		ID:    "AdminGoroutineLabel",
		Other: "Goroutine",
	}
	AdminMemoryUsedLabel = &i18n.Message{
		ID:    "AdminMemoryUsedLabel",
		Other: "Memory used",
	}
	AdminSystemMemoryLabel = &i18n.Message{
		ID:    "AdminSystemMemoryLabel",
		Other: "System memory",
	}
	AdminCPUCoreLabel = &i18n.Message{
		ID:    "AdminCPUCoreLabel",
		Other: "CPU core",
	}
	AdminConfigurationTitle = &i18n.Message{
		ID:    "AdminConfigurationTitle",
		Other: "Configuration",
	}
	AdminAdminsLabel = &i18n.Message{
		ID:    "AdminAdminsLabel",
		Other: "Admins",
	}
	AdminWhitelistLabel = &i18n.Message{
		ID:    "AdminWhitelistLabel",
		Other: "Whitelist",
	}
	AdminBannedChatsLabel = &i18n.Message{
		ID:    "AdminBannedChatsLabel",
		Other: "Banned chats",
	}
	AdminMutedChatsLabel = &i18n.Message{
		ID:    "AdminMutedChatsLabel",
		Other: "Muted chats",
	}
	AdminConcurrentLabel = &i18n.Message{
		ID:    "AdminConcurrentLabel",
		Other: "Concurrent operations",
	}
	AdminMaxDurationLabel = &i18n.Message{
		ID:    "AdminMaxDurationLabel",
		Other: "Maximum duration",
	}
	AdminMaxFileSizeLabel = &i18n.Message{
		ID:    "AdminMaxFileSizeLabel",
		Other: "Maximum file size",
	}
	AdminCacheLabel = &i18n.Message{
		ID:    "AdminCacheLabel",
		Other: "Cache",
	}
	AdminLogLevelLabel = &i18n.Message{
		ID:    "AdminLogLevelLabel",
		Other: "Log level",
	}
	AdminTimeLabel = &i18n.Message{
		ID:    "AdminTimeLabel",
		Other: "Time",
	}
	AdminContentLabel = &i18n.Message{
		ID:    "AdminContentLabel",
		Other: "content",
	}
	AdminGroupLabel = &i18n.Message{
		ID:    "AdminGroupLabel",
		Other: "group",
	}
	AdminRecordsWord = &i18n.Message{
		ID:    "AdminRecordsWord",
		Other: "records",
	}
	AdminFirstPageButton = &i18n.Message{
		ID:    "AdminFirstPageButton",
		Other: "First page",
	}
	AdminPreviousPageButton = &i18n.Message{
		ID:    "AdminPreviousPageButton",
		Other: "Previous",
	}
	AdminNextPageButton = &i18n.Message{
		ID:    "AdminNextPageButton",
		Other: "Next",
	}
	AdminLastPageButton = &i18n.Message{
		ID:    "AdminLastPageButton",
		Other: "Last page",
	}
	AdminNoUsers = &i18n.Message{
		ID:    "AdminNoUsers",
		Other: "No users have been recorded yet.",
	}
	AdminNoGroups = &i18n.Message{
		ID:    "AdminNoGroups",
		Other: "No groups have been recorded yet.",
	}
	AdminMutedUsers = &i18n.Message{
		ID:    "AdminMutedUsers",
		Other: "Muted Users",
	}
	AdminBannedUsers = &i18n.Message{
		ID:    "AdminBannedUsers",
		Other: "Banned Users",
	}
	AdminMutedGroups = &i18n.Message{
		ID:    "AdminMutedGroups",
		Other: "Muted Groups",
	}
	AdminBannedGroups = &i18n.Message{
		ID:    "AdminBannedGroups",
		Other: "Banned Groups",
	}
	AdminNoMutedUsers = &i18n.Message{
		ID:    "AdminNoMutedUsers",
		Other: "No muted users right now.",
	}
	AdminNoBannedUsers = &i18n.Message{
		ID:    "AdminNoBannedUsers",
		Other: "No banned users yet.",
	}
	AdminNoMutedGroups = &i18n.Message{
		ID:    "AdminNoMutedGroups",
		Other: "No muted groups right now.",
	}
	AdminNoBannedGroups = &i18n.Message{
		ID:    "AdminNoBannedGroups",
		Other: "No banned groups yet.",
	}
	AdminUserProfileTitle = &i18n.Message{
		ID:    "AdminUserProfileTitle",
		Other: "User Profile",
	}
	AdminGroupProfileTitle = &i18n.Message{
		ID:    "AdminGroupProfileTitle",
		Other: "Group Profile",
	}
	AdminIDLabel = &i18n.Message{
		ID:    "AdminIDLabel",
		Other: "ID",
	}
	AdminUsernameLabel = &i18n.Message{
		ID:    "AdminUsernameLabel",
		Other: "Username",
	}
	AdminStatusLabel = &i18n.Message{
		ID:    "AdminStatusLabel",
		Other: "Status",
	}
	AdminRegisteredLabel = &i18n.Message{
		ID:    "AdminRegisteredLabel",
		Other: "Registered",
	}
	AdminLastSeenLabel = &i18n.Message{
		ID:    "AdminLastSeenLabel",
		Other: "Last seen",
	}
	AdminLastActiveLabel = &i18n.Message{
		ID:    "AdminLastActiveLabel",
		Other: "Last active",
	}
	AdminActivityTitle = &i18n.Message{
		ID:    "AdminActivityTitle",
		Other: "Activity",
	}
	AdminPlatformsTitle = &i18n.Message{
		ID:    "AdminPlatformsTitle",
		Other: "Platforms",
	}
	AdminNoRecords = &i18n.Message{
		ID:    "AdminNoRecords",
		Other: "No records.",
	}
	AdminRecentDownloads = &i18n.Message{
		ID:    "AdminRecentDownloads",
		Other: "Recent Downloads",
	}
	AdminRecentUsersTitle = &i18n.Message{
		ID:    "AdminRecentUsersTitle",
		Other: "Recent Private Users",
	}
	AdminRecentGroupsTitle = &i18n.Message{
		ID:    "AdminRecentGroupsTitle",
		Other: "Recent Groups",
	}
	AdminSummaryButton = &i18n.Message{
		ID:    "AdminSummaryButton",
		Other: "Summary",
	}
	AdminDownloadLabel = &i18n.Message{
		ID:    "AdminDownloadLabel",
		Other: "Downloads",
	}
	AdminSizeLabel = &i18n.Message{
		ID:    "AdminSizeLabel",
		Other: "Size",
	}
	AdminNoDownloads = &i18n.Message{
		ID:    "AdminNoDownloads",
		Other: "No downloads yet.",
	}
	AdminRecentErrorsTitle = &i18n.Message{
		ID:    "AdminRecentErrorsTitle",
		Other: "Recent Errors",
	}
	AdminNoErrors = &i18n.Message{
		ID:    "AdminNoErrors",
		Other: "No errors recorded.",
	}
	AdminOccurrencesLabel = &i18n.Message{
		ID:    "AdminOccurrencesLabel",
		Other: "Occurrences",
	}
	AdminUnknownGroup = &i18n.Message{
		ID:    "AdminUnknownGroup",
		Other: "Unknown Group",
	}
	AdminTimeUnknown = &i18n.Message{
		ID:    "AdminTimeUnknown",
		Other: "unknown",
	}
	AdminTimeJustNow = &i18n.Message{
		ID:    "AdminTimeJustNow",
		Other: "just now",
	}
	AdminTimeMinutesAgo = &i18n.Message{
		ID:    "AdminTimeMinutesAgo",
		Other: "{{.Count}} min ago",
	}
	AdminTimeHoursAgo = &i18n.Message{
		ID:    "AdminTimeHoursAgo",
		Other: "{{.Count}} hr ago",
	}
	AdminReasonLabel = &i18n.Message{
		ID:    "AdminReasonLabel",
		Other: "Reason",
	}
	AdminUnknownUser = &i18n.Message{
		ID:    "AdminUnknownUser",
		Other: "unknown",
	}
	AdminProtectedUser = &i18n.Message{
		ID:    "AdminProtectedUser",
		Other: "Protected User",
	}
	AdminAdminsCannotBan = &i18n.Message{
		ID:    "AdminAdminsCannotBan",
		Other: "Admins cannot be banned.",
	}
	AdminAdminsCannotMute = &i18n.Message{
		ID:    "AdminAdminsCannotMute",
		Other: "Admins cannot be muted.",
	}
	AdminBanConfirmTitle = &i18n.Message{
		ID:    "AdminBanConfirmTitle",
		Other: "Ban Confirmation",
	}
	AdminGroupBanConfirmTitle = &i18n.Message{
		ID:    "AdminGroupBanConfirmTitle",
		Other: "Group Ban Confirmation",
	}
	AdminBanButton = &i18n.Message{
		ID:    "AdminBanButton",
		Other: "Ban",
	}
	AdminUnbanButton = &i18n.Message{
		ID:    "AdminUnbanButton",
		Other: "Unban",
	}
	AdminMute1hButton = &i18n.Message{
		ID:    "AdminMute1hButton",
		Other: "Mute 1h",
	}
	AdminMute24hButton = &i18n.Message{
		ID:    "AdminMute24hButton",
		Other: "Mute 24h",
	}
	AdminUnmuteButton = &i18n.Message{
		ID:    "AdminUnmuteButton",
		Other: "Unmute",
	}
	AdminGroupBanButton = &i18n.Message{
		ID:    "AdminGroupBanButton",
		Other: "Ban Group",
	}
	AdminGroupUnbanButton = &i18n.Message{
		ID:    "AdminGroupUnbanButton",
		Other: "Unban Group",
	}
	AdminGroupMute1hButton = &i18n.Message{
		ID:    "AdminGroupMute1hButton",
		Other: "Mute 1h",
	}
	AdminGroupMute24hButton = &i18n.Message{
		ID:    "AdminGroupMute24hButton",
		Other: "Mute 24h",
	}
	AdminCleanupTitle = &i18n.Message{
		ID:    "AdminCleanupTitle",
		Other: "Database Cleanup",
	}
	AdminCleanupSelectCategory = &i18n.Message{
		ID:    "AdminCleanupSelectCategory",
		Other: "Select the category you want to clean.",
	}
	AdminCleanupUsersButton = &i18n.Message{
		ID:    "AdminCleanupUsersButton",
		Other: "Clear All Users",
	}
	AdminCleanupGroupsButton = &i18n.Message{
		ID:    "AdminCleanupGroupsButton",
		Other: "Clear All Groups",
	}
	AdminCleanupDownloadsButton = &i18n.Message{
		ID:    "AdminCleanupDownloadsButton",
		Other: "Clear Download History",
	}
	AdminCleanupErrorsButton = &i18n.Message{
		ID:    "AdminCleanupErrorsButton",
		Other: "Clear Errors",
	}
	AdminErrorsButton = &i18n.Message{
		ID:    "AdminErrorsButton",
		Other: "Errors",
	}
	AdminDatabaseCleanupButton = &i18n.Message{
		ID:    "AdminDatabaseCleanupButton",
		Other: "Database Cleanup",
	}
	AdminCleanupCleaning = &i18n.Message{
		ID:    "AdminCleanupCleaning",
		Other: "Cleaning...",
	}
	AdminCleanupUsersSuccess = &i18n.Message{
		ID:    "AdminCleanupUsersSuccess",
		Other: "users were removed from the database.",
	}
	AdminCleanupGroupsSuccess = &i18n.Message{
		ID:    "AdminCleanupGroupsSuccess",
		Other: "groups were removed from the database.",
	}
	AdminCleanupDownloadsSuccess = &i18n.Message{
		ID:    "AdminCleanupDownloadsSuccess",
		Other: "Download history was cleared.",
	}
	AdminCleanupErrorsSuccess = &i18n.Message{
		ID:    "AdminCleanupErrorsSuccess",
		Other: "Error records were cleared.",
	}
	AdminCleanupErrorPrefix = &i18n.Message{
		ID:    "AdminCleanupErrorPrefix",
		Other: "Error:",
	}
	StatusActive = &i18n.Message{
		ID:    "StatusActive",
		Other: "Active",
	}
	StatusBanned = &i18n.Message{
		ID:    "StatusBanned",
		Other: "Banned",
	}
	StatusMutedRemaining = &i18n.Message{
		ID:    "StatusMutedRemaining",
		Other: "Muted · remaining: {{.Duration}}",
	}
	StatusUnknown = &i18n.Message{
		ID:    "StatusUnknown",
		Other: "unknown",
	}
)
