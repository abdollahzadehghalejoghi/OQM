# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

