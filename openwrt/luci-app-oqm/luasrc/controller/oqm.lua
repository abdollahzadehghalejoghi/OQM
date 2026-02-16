module("luci.controller.oqm", package.seeall)

function index()
    entry({"admin", "network", "oqm"}, template("oqm/index"), _("Quota Manager"), 60)
end
