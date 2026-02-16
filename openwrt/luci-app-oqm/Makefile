include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-oqm
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_LICENSE:=MIT
PKG_MAINTAINER:=Your Name <your.email@example.com>

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-oqm
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=LuCI Support for OQM (OpenWrt Quota Manager)
  DEPENDS:=+oqm +luci-base
  PKGARCH:=all
endef

define Package/luci-app-oqm/description
  LuCI web interface for OQM (OpenWrt Quota Manager).
  Provides a user-friendly interface to manage user quotas,
  monitor traffic, and control internet access.
endef

define Build/Compile
endef

define Package/luci-app-oqm/install
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/controller
	$(INSTALL_DATA) ./luasrc/controller/oqm.lua $(1)/usr/lib/lua/luci/controller/

	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/view/oqm
	$(INSTALL_DATA) ./luasrc/view/oqm/index.htm $(1)/usr/lib/lua/luci/view/oqm/
endef

$(eval $(call BuildPackage,luci-app-oqm))
