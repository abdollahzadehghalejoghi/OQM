# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [1.1.1] - 2026-02-22

### Added
- 📊 **Enhanced Block/Unblock Notifications**: Show used volume and quota in notifications
  - Block notifications now display used MB and quota MB
  - Unblock notifications now display used MB and quota MB
  - Helps users understand exactly how much they've used and their limit
  - Applies to both individual device and group quota notifications

### Changed
- 🔧 **Centralized Version Management**: Single source of truth for version number
  - Created `VERSION` file to store current version
  - Makefile now reads version from VERSION file
  - Version properly injected at build time via ldflags
  - OpenWrt Makefiles updated to use consistent version
  - Ensures all components display the same version number

## [1.1.0] - 2026-02-21

### Added
- 🔔 **Reset Notifications**: Send notifications to users and admin when usage is reset
  - Manual reset via CLI (`oqm reset-usage`) now sends notifications
  - Manual reset via Web UI now sends notifications
  - Scheduled monthly reset sends notifications to affected users
  - Individual user notifications sent to their Telegram/Bale chat ID
  - Admin receives system event notification with reset summary
- 📊 **Reset Tracking**: New notification method `NotifyUserReset` for user-specific reset alerts
- 📊 **Enhanced Block/Unblock Notifications**: Show used volume and quota in notifications
  - Block notifications now display used MB and quota MB
  - Unblock notifications now display used MB and quota MB
  - Helps users understand exactly how much they've used and their limit
  - Applies to both individual device and group quota notifications

### Fixed
- ✅ **Quota Warning Spam Prevention**: 80% quota warning now only sent once
  - Added `DeviceWarningNotificationSent` flag to track device quota warnings
  - Added `GroupWarningNotificationSent` flag to track group quota warnings
  - Warning flags automatically reset when usage drops below 80%
  - Warning flags reset when usage is manually or automatically reset
  - Prevents duplicate notifications during monitoring loops
- ✅ **Individual User Quota Check**: Fixed quota checking for users with username but no group quota
  - Individual users with username field now properly checked for device quota
  - Users without group quota no longer skipped in quota checks
- ✅ **NFTables Cleanup on Delete**: Deleted users now properly removed from nftables
  - User IPs removed from nftables monitoring sets when deleted
  - Prevents orphaned entries in nftables after user deletion

### Technical Details
- **Storage**: Added `device_warning_notification_sent` and `group_warning_notification_sent` fields to User model
- **Notifications**: Enhanced notification system to support individual user reset alerts
- **Daemon**: Improved quota monitoring logic to track notification state
- **Backward Compatible**: New fields are optional and default to false for existing users

## [1.0.0] - 2026-02-16

### Added
- 🎯 **Dual Quota System**: Device quota + Group quota (shared among devices)
- 🤖 **Multi-Messenger Support**: Telegram AND Bale notifications
- 🔄 **Smart Auto-Unblock**: Automatically unblock users when quota is increased
- 👥 **Advanced Group Management**: Group unlimited devices with shared quotas
- 🆔 **MAC Address Tracking**: Track devices by MAC with automatic IP resolution via ARP
- 📡 **DHCP Integration**: Auto-discover and add new devices from DHCP leases
- 📊 **Enhanced Web UI**:
  - Real-time charts for top users
  - Group expansion/collapse in user table
  - Bot type selector (Telegram/Bale/Custom)
  - Larger, more user-friendly modals
  - Online/Offline status indicators
  - Test bot connection directly from settings
- 📱 **Dual Notification Mode**: Send notifications to both user AND admin simultaneously
- 🔧 **Group Quota Editing**: Edit group settings separately from device settings
- 🎨 **Modern UI**: Beautiful dashboard with tailored colors for device vs group quotas

### Changed
- 🔄 **Improved Counter Parsing**: Fixed nftables counter parsing to handle multiple IPs per line
- ⚡ **Immediate Sync on Startup**: Daemon now syncs storage immediately instead of waiting 60 seconds
- 🎯 **Better Quota Logic**: Check quota even for blocked users to enable auto-unblock
- 🔐 **Bot API Fix**: Set base URL before making API calls (fixes Bale connectivity)
- 📝 **Username Handling**: Fixed username selection from dropdown vs input field
- 🎨 **Modal Sizes**: Increased modal sizes for better UX

### Fixed
- ✅ Fixed counter parsing bug where only first IP per line was captured
- ✅ Fixed auto-unblock not working when quota increased
- ✅ Fixed Bale bot connecting to Telegram API instead of Bale
- ✅ Fixed username field being null when editing users
- ✅ Fixed group quota fields being sent when editing individual devices
- ✅ Fixed daemon not checking blocked users for potential unblock
- ✅ Fixed device quota not showing for group members

### Technical Details
- **Backend**: Complete rewrite of quota checking logic
- **Frontend**: React-like state management for better UI responsiveness
- **Storage**: Backward compatible config migration (old fields still work)
- **Notifications**: Unified bot interface supporting multiple messaging platforms

### Migration Notes
- Old `telegram_bot_token` and `telegram_admin_chat_id` fields still work
- New fields: `bot_type`, `bot_token`, `bot_api_base_url`, `admin_chat_id`
- Automatic migration on first config save
- No data loss or manual intervention required

## [0.1.0] - Initial Release

### Added
- Basic traffic monitoring with nftables
- Per-user quota management
- Simple Web UI
- CLI interface
- Telegram notifications
- Auto-blocking on quota exceeded

