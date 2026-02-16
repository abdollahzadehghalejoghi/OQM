# LuCI App for OQM (OpenWrt Quota Manager)

This package integrates OQM into the LuCI web interface.

## Installation

### Method 1: Manual Installation (Quick)

1. Copy the files directly to your OpenWrt router:

```bash
# On your router
mkdir -p /usr/lib/lua/luci/controller
mkdir -p /usr/lib/lua/luci/view/oqm

# From your development machine
scp openwrt/luci-app-oqm/luasrc/controller/oqm.lua root@192.168.1.1:/usr/lib/lua/luci/controller/
scp openwrt/luci-app-oqm/luasrc/view/oqm/index.htm root@192.168.1.1:/usr/lib/lua/luci/view/oqm/
```

2. Clear LuCI cache:

```bash
ssh root@192.168.1.1
rm -rf /tmp/luci-*
/etc/init.d/rpcd restart
```

3. Refresh your browser and navigate to: **Network → Quota Manager**

### Method 2: Build as OpenWrt Package

If you're building a custom OpenWrt image:

1. Copy the entire `luci-app-oqm` directory to your OpenWrt buildroot:

```bash
cp -r openwrt/luci-app-oqm <openwrt-buildroot>/package/luci-app-oqm
```

2. Build the package:

```bash
cd <openwrt-buildroot>
make package/luci-app-oqm/compile
```

3. Install the generated `.ipk` file on your router.

## Features

- Embedded OQM web interface directly in LuCI
- Accessible from: **Network → Quota Manager**
- No need to remember port 8080
- Seamless integration with OpenWrt's web interface

## Notes

- The OQM daemon must be running on port 8080
- The iframe will auto-refresh every 30 seconds
- All OQM features are available through the embedded interface
