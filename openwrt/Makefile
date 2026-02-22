# OpenWrt Makefile for OQM
include $(TOPDIR)/rules.mk

PKG_NAME:=oqm
PKG_VERSION:=1.1.1
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/abdollahzadehghalejoghi/oqm
PKG_SOURCE_VERSION:=main
PKG_MIRROR_HASH:=skip

PKG_MAINTAINER:=Amirhossein Abdollahzadeh <your-email@example.com>
PKG_LICENSE:=MIT
PKG_LICENSE_FILES:=LICENSE

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_USE_MIPS16:=0

GO_PKG:=github.com/abdollahzadehghalejoghi/oqm
GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/oqm
GO_PKG_LDFLAGS:=-s -w -X main.version=$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/oqm
  SECTION:=net
  CATEGORY:=Network
  TITLE:=OpenWrt Quota Manager
  URL:=https://github.com/abdollahzadehghalejoghi/oqm
  DEPENDS:=+nftables +kmod-nft-core
  PKGARCH:=all
endef

define Package/oqm/description
  OQM (OpenWrt Quota Manager) is a powerful and lightweight quota
  management system for OpenWrt routers. It monitors network usage,
  enforces quota limits, and provides a web-based dashboard.

  Features:
  - Per-user quota management
  - Automatic blocking when quota exceeded
  - Telegram notifications
  - Web UI dashboard
  - Multi-device support per user
  - Scheduled quota resets
endef

define Package/oqm/conffiles
/etc/oqm/data.json
endef

define Build/Configure
	$(call GoPackage/Build/Configure)
endef

define Build/Compile
	$(call GoPackage/Build/Compile)
endef

define Package/oqm/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/oqm $(1)/usr/bin/

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/oqm $(1)/etc/init.d/

	$(INSTALL_DIR) $(1)/etc/oqm
	$(INSTALL_DATA) ./files/etc/oqm/data.json $(1)/etc/oqm/
endef

define Package/oqm/postinst
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	echo "Enabling and starting OQM service..."
	/etc/init.d/oqm enable
	/etc/init.d/oqm start
fi
exit 0
endef

define Package/oqm/prerm
#!/bin/sh
if [ -z "$${IPKG_INSTROOT}" ]; then
	echo "Stopping OQM service..."
	/etc/init.d/oqm stop
	/etc/init.d/oqm disable
fi
exit 0
endef

$(eval $(call GoBinPackage,oqm))
$(eval $(call BuildPackage,oqm))
