'use strict';

var dashboard = angular.module('dashboard-fe', ["highcharts-ng", "ui.bootstrap"]);

function genXAuth(name) {
    return sha256("Codis-XAuth-[" + name + "]").substring(0, 32);
}

function concatUrl(base, name) {
    if (name) {
        return encodeURI(base + "?forward=" + name);
    } else {
        return encodeURI(base);
    }
}

function padInt2Str(num, size) {
    var s = num + "";
    while (s.length < size) s = "0" + s;
    return s;
}

function toJsonHtml(obj) {
    var json = angular.toJson(obj, 4);
    json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    json = json.replace(/ /g, '&nbsp;');
    json = json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
        var cls = 'number';
        if (/^"/.test(match)) {
            if (/:$/.test(match)) {
                cls = 'key';
            } else {
                cls = 'string';
            }
        } else if (/true|false/.test(match)) {
            cls = 'boolean';
        } else if (/null/.test(match)) {
            cls = 'null';
        }
        return '<span class="' + cls + '">' + match + '</span>';
    });
    return json;
}

function humanSize(size) {
    if (size < 1024) {
        return size + " B";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " KB";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " MB";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " GB";
    }
    size /= 1024;
    return size.toFixed(2) + " TB";
}

function newChatsOpsConfig() {
    return {
        options: {
            chart: {
                useUTC: false,
                type: 'spline',
            },
        },
        series: [{
            color: '#d82b28',
            lineWidth: 1.5,
            states: {
                hover: {
                    enabled: false,
                }
            },
            showInLegend: false,
            marker: {
                enabled: true,
                symbol: 'circle',
                radius: 2,
            },
            name: 'OP/s',
            data: [],
        }],
        title: {
            style: {
                display: 'none',
            }
        },
        xAxis: {
            type: 'datetime',
            title: {
                style: {
                    display: 'none',
                }
            },
            labels: {
                formatter: function () {
                    var d = new Date(this.value);
                    return padInt2Str(d.getHours(), 2) + ":" + padInt2Str(d.getMinutes(), 2) + ":" + padInt2Str(d.getSeconds(), 2);
                }
            },
        },
        yAxis: {
            min: 0,
            title: {
                style: {
                    display: 'none',
                }
            },
        },
    };
}

function renderSlotsCharts(slots_array) {
    var groups = {};
    var counts = {};
    var n = slots_array.length;
    for (var i = 0; i < n; i++) {
        var slot = slots_array[i];
        groups[slot.group_id] = true;
        if (slot.action.target_id) {
            groups[slot.action.target_id] = true;
        }
        if (counts[slot.group_id]) {
            counts[slot.group_id]++;
        } else {
            counts[slot.group_id] = 1;
        }
    }
    var series = [];
    for (var g in groups) {
        var xaxis = 2;
        if (g == 0) {
            xaxis = 0;
        }
        var s = {name: 'Group-' + g + ':' + (counts[g] == undefined ? 0 : counts[g]), data: [], group_id: g};
        for (var beg = 0, end = 0; end <= n; end++) {
            if (end == n || slots_array[end].group_id != g) {
                if (beg < end) {
                    s.data.push({x: xaxis, beg: beg, end: end - 1, low: beg, high: end, group_id: g});
                }
                beg = end + 1;
            }
        }
        xaxis = 1;
        for (var beg = 0, end = 0; end <= n; end++) {
            if (end == n || !(slots_array[end].action.target_id && slots_array[end].action.target_id == g)) {
                if (beg < end) {
                    s.data.push({x: xaxis, beg: beg, end: end - 1, low: beg, high: end, group_id: g});
                }
                beg = end + 1;
            }
        }
        s.data.sort(function (a, b) {
            return a.x - b.x;
        });
        series.push(s);
    }
    series.sort(function (a, b) {
        return a.group_id - b.group_id;
    });
    new Highcharts.Chart({
        chart: {
            renderTo: 'slots_charts',
            type: 'columnrange',
            inverted: true,
        },
        title: {
            style: {
                display: 'none',
            }
        },
        xAxis: {
            categories: ['Offline', 'Migrating', 'Default'],
            min: 0,
            max: 2,
        },
        yAxis: {
            min: 0,
            max: 1024,
            tickInterval: 64,
            title: {
                style: {
                    display: 'none',
                }
            },
        },
        legend: {
            enabled: true,
            itemWidth: 140,
        },
        plotOptions: {
            columnrange: {
                grouping: false
            },
            series: {
                animation: false,
                events: {
                    legendItemClick: function () {
                        return false;
                    },
                }
            },
        },
        credits: {
            enabled: false
        },
        tooltip: {
            formatter: function () {
                switch (this.point.x) {
                case 0:
                    return '<b>Slot-[' + this.point.beg + "," + this.point.end + "]</b> are <b>Offline</b>";
                case 1:
                    return '<b>Slot-[' + this.point.beg + "," + this.point.end + "]</b> will be moved to <b>Group-[" + this.point.group_id + "]</b>";
                case 2:
                    return '<b>Slot-[' + this.point.beg + "," + this.point.end + "]</b> --> <b>Group-[" + this.point.group_id + "]</b>";
                }
            }
        },
        series: series,
    });
}

function processProxyStats(codis_stats) {
    var proxy_array = codis_stats.proxy.models;
    var proxy_stats = codis_stats.proxy.stats;
    var qps = 0, sessions = 0;
    for (var i = 0; i < proxy_array.length; i++) {
        var p = proxy_array[i];
        var s = proxy_stats[p.token];
        p.sessions = "NA";
        p.commands = "NA";
        p.switched = false;
        p.primary_only = false;
        if (!s) {
            p.status = "PENDING";
        } else if (s.timeout) {
            p.status = "TIMEOUT";
        } else if (s.error) {
            p.status = "ERROR";
        } else {
            if (s.stats.online) {
                p.sessions = "total=" + s.stats.sessions.total + ",alive=" + s.stats.sessions.alive;
                p.commands = "total=" + s.stats.ops.total + ",fails=" + s.stats.ops.fails;
                if (s.stats.ops.redis != undefined) {
                    p.commands += ",rsp.errs=" + s.stats.ops.redis.errors;
                }
                p.commands += ",qps=" + s.stats.ops.qps;
                p.status = "HEALTHY";
            } else {
                p.status = "PENDING";
            }
            if (p.admin_addr) {
                p.proxy_link = p.admin_addr + "/proxy"
                p.stats_link = p.admin_addr + "/proxy/stats"
            }
            if (s.stats.sentinels != undefined) {
                if (s.stats.sentinels.switched != undefined) {
                    p.switched = s.stats.sentinels.switched;
                }
            }
            if (s.stats.backend != undefined) {
                if (s.stats.backend.primary_only != undefined) {
                    p.primary_only = s.stats.backend.primary_only;
                }
            }
            qps += s.stats.ops.qps;
            sessions += s.stats.sessions.alive;
        }
    }
    return {proxy_array: proxy_array, qps: qps, sessions: sessions};
}

function processSentinels(codis_stats, group_stats, codis_name) {
    var ha = codis_stats.sentinels;
    var out_of_sync = false;
    var servers = [];
    if (ha.model != undefined) {
        if (ha.model.servers == undefined) {
            ha.model.servers = []
        }
        for (var i = 0; i < ha.model.servers.length; i ++) {
            var x = {server: ha.model.servers[i]};
            var s = ha.stats[x.server];
            x.runid_error = "";
            if (!s) {
                x.status = "PENDING";
            } else if (s.timeout) {
                x.status = "TIMEOUT";
            } else if (s.error) {
                x.status = "ERROR";
            } else {
                x.masters = 0;
                x.masters_down = 0;
                x.slaves = 0;
                x.sentinels = 0;
                var masters = s.stats["sentinel_masters"];
                if (masters != undefined) {
                    for (var j = 0; j < masters; j ++) {
                        var record = s.stats["master" + j];
                        if (record != undefined) {
                            var pairs = record.split(",");
                            var dict = {};
                            for (var t = 0; t < pairs.length; t ++) {
                                var ss = pairs[t].split("=");
                                if (ss.length == 2) {
                                    dict[ss[0]] = ss[1];
                                }
                            }
                            var name = dict["name"];
                            if (name == undefined) {
                                continue;
                            }
                            if (name.lastIndexOf(codis_name) != 0) {
                                continue;
                            }
                            if (name.lastIndexOf("-") != codis_name.length) {
                                continue;
                            }
                            x.masters ++;
                            if (dict["status"] != "ok") {
                                x.masters_down ++;
                            }
                            x.slaves += parseInt(dict["slaves"]);
                            x.sentinels += parseInt(dict["sentinels"]);
                        }
                    }
                }
                x.status_text = "masters=" + x.masters;
                x.status_text += ",down=" + x.masters_down;
                var avg = 0;
                if (x.slaves == 0) {
                    avg = 0;
                } else {
                    avg = Number(x.slaves) / x.masters;
                }
                x.status_text += ",slaves=" + avg.toFixed(2);
                if (x.sentinels == 0) {
                    avg = 0;
                } else {
                    avg = Number(x.sentinels) / x.masters;
                }
                x.status_text += ",sentinels=" + avg.toFixed(2);

                if (s.sentinel != undefined) {
                    var group_array = group_stats.group_array;
                    for (var t in group_array) {
                        var g = group_array[t];
                        var d = s.sentinel[codis_name + "-" + g.id];
                        var runids = {};
                        if (d != undefined) {
                            if (d.master != undefined) {
                                var o = d.master;
                                runids[o["runid"]] = o["ip"] + ":" + o["port"];
                            }
                            if (d.slaves != undefined) {
                                for (var j = 0; j < d.slaves.length; j ++) {
                                    var o = d.slaves[j];
                                    runids[o["runid"]] = o["ip"] + ":" + o["port"];
                                }
                            }
                        }
                        for (var runid in runids) {
                            if (g.runids[runid] === undefined) {
                                x.runid_error = "[+]group=" + g.id + ",server=" + runids[runid] + ",runid="
                                    + ((runid != "") ? runid : "NA");
                            }
                        }
                        for (var runid in g.runids) {
                            if (runids[runid] === undefined) {
                                x.runid_error = "[-]group=" + g.id + ",server=" + g.runids[runid] + ",runid=" + runid;
                            }
                        }
                    }
                }
            }
            servers.push(x);
        }
        out_of_sync = ha.model.out_of_sync;
    }
    var masters = ha.masters;
    if (masters == undefined) {
        masters = {};
    }
    return {servers:servers, masters:masters, out_of_sync: out_of_sync}
}

function alertAction(text, callback) {
    BootstrapDialog.show({
        title: "Warning !!",
        message: text,
        closable: true,
        buttons: [{
            label: "OK",
            cssClass: "btn-primary",
            action: function (dialog) {
                dialog.close();
                callback();
            },
        }, {
            label: "CANCEL",
            action: function(dialogItself){
                dialogItself.close();
            }
        }],
    });
}

function alertAction2(text, callback) {
    BootstrapDialog.show({
        title: "Warning !!",
        type: "type-danger",
        message: text,
        closable: true,
        buttons: [{
            label: "JUST DO IT",
            cssClass: "btn-danger",
            action: function (dialog) {
                dialog.close();
                callback();
            },
        }, {
            label: "CANCEL",
            action: function(dialogItself){
                dialogItself.close();
            }
        }],
    });
}

function alertErrorResp(failedResp) {
    var text = "error response";
    if (failedResp.status != 1500 && failedResp.status != 800) {
        text = failedResp.data.toString();
    } else {
        text = toJsonHtml(failedResp.data);
    }
    BootstrapDialog.alert({
        title: "Error !!",
        type: "type-danger",
        closable: true,
        message: text,
    });
}

function isValidInput(text) {
    return text && text != "" && text != "NA";
}

function processGroupStats(codis_stats) {
    var group_array = codis_stats.group.models;
    var group_stats = codis_stats.group.stats;
    var keys = 0, memory = 0;
    var dbkeyRegexp = /db\d+/
    for (var i = 0; i < group_array.length; i++) {
        var g = group_array[i];
        if (g.promoting.state) {
            g.ispromoting = g.promoting.state != "";
            if (g.promoting.index) {
                g.ispromoting_index = g.promoting.index;
            } else {
                g.ispromoting_index = 0;
            }
        } else {
            g.ispromoting = false;
            g.ispromoting_index = -1;
        }
        g.runids = {}
        g.canremove = (g.servers.length == 0);
        for (var j = 0; j < g.servers.length; j++) {
            var x = g.servers[j];
            var s = group_stats[x.server];
            x.keys = [];
            x.memory = "NA";
            x.maxmem = "NA";
            x.master = "NA";
            if (j == 0) {
                x.master_expect = "NO:ONE";
            } else {
                x.master_expect = g.servers[0].server;
            }
            if (!s) {
                x.status = "PENDING";
            } else if (s.timeout) {
                x.status = "TIMEOUT";
            } else if (s.error) {
                x.status = "ERROR";
            } else {
                for (var field in s.stats) {
                    if (dbkeyRegexp.test(field)) {
                        var v = parseInt(s.stats[field].split(",")[0].split("=")[1], 10);
                        if (j == 0) {
                            keys += v;
                        }
                        x.keys.push(field+ ":" + s.stats[field]);
                    }
                }
                if (s.stats["used_memory"]) {
                    var v = parseInt(s.stats["used_memory"], 10);
                    if (j == 0) {
                        memory += v;
                    }
                    x.memory = humanSize(v);
                }
                if (s.stats["maxmemory"]) {
                    var v = parseInt(s.stats["maxmemory"], 10);
                    if (v == 0) {
                        x.maxmem = "INF."
                    } else {
                        x.maxmem = humanSize(v);
                    }
                }
                if (s.stats["master_addr"]) {
                    x.master = s.stats["master_addr"] + ":" + s.stats["master_link_status"];
                } else {
                    x.master = "NO:ONE";
                }
                if (j == 0) {
                    x.master_status = (x.master == "NO:ONE");
                } else {
                    x.master_status = (x.master == g.servers[0].server + ":up");
                }
                g.runids[s.stats["run_id"]] = x.server;
            }
            if (g.ispromoting) {
                x.canremove = false;
                x.canpromote = false;
                x.ispromoting = (j == g.ispromoting_index);
            } else {
                x.canremove = (j != 0 || g.servers.length <= 1);
                x.canpromote = j != 0;
                x.ispromoting = false;
            }
            if (x.action.state) {
                if (x.action.state != "pending") {
                    x.canslaveof = "create";
                    x.actionstate = x.action.state;
                } else {
                    x.canslaveof = "remove";
                    x.actionstate = x.action.state + ":" + x.action.index;
                }
            } else {
                x.canslaveof = "create";
                x.actionstate = "";
            }
            x.server_text = x.server;
        }
    }
    return {group_array: group_array, keys: keys, memory: memory};
}

dashboard.config(['$interpolateProvider',
    function ($interpolateProvider) {
        $interpolateProvider.startSymbol('[[');
        $interpolateProvider.endSymbol(']]');
    }
]);

dashboard.config(['$httpProvider', function ($httpProvider) {
    $httpProvider.defaults.useXDomain = true;
    delete $httpProvider.defaults.headers.common['X-Requested-With'];
}]);

dashboard.controller('MainCodisCtrl', ['$scope', '$http', '$uibModal', '$timeout',
    function ($scope, $http, $uibModal, $timeout) {
        Highcharts.setOptions({
            global: {
                useUTC: false,
            },
            exporting: {
                enabled: false,
            },
        });
        $scope.chart_ops = newChatsOpsConfig();

        $scope.refresh_interval = 3;

        $scope.resetOverview = function () {
            $scope.codis_name = "NA";
            $scope.codis_addr = "NA";
            $scope.codis_coord_name = "Coordinator";
            $scope.codis_coord_addr = "NA";
            $scope.codis_qps = "NA";
            $scope.codis_sessions = "NA";
            $scope.redis_mem = "NA";
            $scope.redis_keys = "NA";
            $scope.slots_array = [];
            $scope.proxy_array = [];
            $scope.group_array = [];
            $scope.slots_actions = [];
            $scope.chart_ops.series[0].data = [];
            $scope.slots_action_interval = "NA";
            $scope.slots_action_disabled = "NA";
            $scope.slots_action_failed = false;
            $scope.slots_action_remain = 0;
            $scope.sentinel_servers = [];
            $scope.sentinel_out_of_sync = false;
        }
        $scope.resetOverview();

        $http.get('/list').then(function (resp) {
            $scope.codis_list = resp.data;
        });

        $scope.selectCodisInstance = function (selected) {
            if ($scope.codis_name == selected) {
                return;
            }
            $scope.resetOverview();
            $scope.codis_name = selected;
            var url = concatUrl("/topom", selected);
            $http.get(url).then(function (resp) {
                if ($scope.codis_name != selected) {
                    return;
                }
                var overview = resp.data;
                $scope.codis_addr = overview.model.admin_addr;
                $scope.codis_coord_name = "[" + overview.config.coordinator_name.charAt(0).toUpperCase() + overview.config.coordinator_name.slice(1) + "]";
                $scope.codis_coord_addr = overview.config.coordinator_addr;
                $scope.updateStats(overview.stats);
            });
        }

        $scope.updateStats = function (codis_stats) {
            var proxy_stats = processProxyStats(codis_stats);
            var group_stats = processGroupStats(codis_stats);
            var sentinel = processSentinels(codis_stats, group_stats, $scope.codis_name);

            var merge = function(obj1, obj2) {
                if (obj1 === null || obj2 === null) {
                    return obj2;
                }
                if (Array.isArray(obj1)) {
                    if (obj1.length != obj2.length) {
                        return obj2;
                    }
                    for (var i = 0; i < obj1.length; i ++) {
                        obj1[i] = merge(obj1[i], obj2[i]);
                    }
                    return obj1;
                }
                if (typeof obj1 === "object") {
                    for (var k in obj1) {
                        if (obj2[k] === undefined) {
                            delete obj1[k];
                        }
                    }
                    for (var k in obj2) {
                        obj1[k] = merge(obj1[k], obj2[k]);
                    }
                    return obj1;
                }
                return obj2;
            }

            $scope.codis_qps = proxy_stats.qps;
            $scope.codis_sessions = proxy_stats.sessions;
            $scope.redis_mem = humanSize(group_stats.memory);
            $scope.redis_keys = group_stats.keys.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
            $scope.slots_array = merge($scope.slots_array, codis_stats.slots);
            $scope.proxy_array = merge($scope.proxy_array, proxy_stats.proxy_array);
            $scope.group_array = merge($scope.group_array, group_stats.group_array);
            $scope.slots_actions = [];
            $scope.slots_action_interval = codis_stats.slot_action.interval;
            $scope.slots_action_disabled = codis_stats.slot_action.disabled;
            $scope.slots_action_progress = codis_stats.slot_action.progress.status;
            $scope.sentinel_servers = merge($scope.sentinel_servers, sentinel.servers);
            $scope.sentinel_out_of_sync = sentinel.out_of_sync;

            for (var i = 0; i < $scope.slots_array.length; i++) {
                var slot = $scope.slots_array[i];
                if (slot.action.state) {
                    $scope.slots_actions.push(slot);
                }
            }

            if ($scope.sentinel_servers.length != 0) {
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var g = $scope.group_array[i];
                    var ha_master = sentinel.masters[g.id];
                    var ha_master_ingroup = false;
                    for (var j = 0; j < g.servers.length; j ++) {
                        var x = g.servers[j];
                        if (ha_master == undefined) {
                            x.ha_status = "ha_undefined";
                            continue;
                        }
                        if (j == 0) {
                            if (x.server == ha_master) {
                                x.ha_status = "ha_master";
                            } else {
                                x.ha_status = "ha_not_master";
                            }
                        } else {
                            if (x.server == ha_master) {
                                x.ha_status = "ha_real_master";
                            } else {
                                x.ha_status = "ha_slave";
                            }
                        }
                        if (x.server == ha_master) {
                            x.server_text = x.server + " [HA]";
                            ha_master_ingroup = true;
                        }
                    }
                    if (ha_master == undefined || ha_master_ingroup) {
                        g.ha_warning = "";
                    } else {
                        g.ha_warning = "[HA: " + ha_master + "]";
                    }
                }
            }

            renderSlotsCharts($scope.slots_array);

            var ops_array = $scope.chart_ops.series[0].data;
            if (ops_array.length >= 10) {
                ops_array.shift();
            }
            ops_array.push({x: new Date(), y: proxy_stats.qps});
            $scope.chart_ops.series[0].data = ops_array;
        }

        $scope.refreshStats = function () {
            var codis_name = $scope.codis_name;
            var codis_addr = $scope.codis_addr;
            if (isValidInput(codis_name) && isValidInput(codis_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/stats/" + xauth, codis_name);
                $http.get(url).then(function (resp) {
                    if ($scope.codis_name != codis_name) {
                        return;
                    }
                    $scope.updateStats(resp.data);
                });
            }
        }

        $scope.createProxy = function (proxy_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(proxy_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/proxy/create/" + xauth + "/" + proxy_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.removeProxy = function (proxy, force) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var prefix = "";
                if (force) {
                    prefix = "[FORCE] ";
                }
                alertAction(prefix + "Remove and Shutdown proxy: " + toJsonHtml(proxy), function () {
                    var xauth = genXAuth(codis_name);
                    var value = 0;
                    if (force) {
                        value = 1;
                    }
                    var url = concatUrl("/api/topom/proxy/remove/" + xauth + "/" + proxy.token + "/" + value, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }

        $scope.reinitProxy = function (proxy) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var confused = [];
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var group = $scope.group_array[i];
                    var ha_real_master = -1;
                    for (var j = 0; j < group.servers.length; j ++) {
                        if (group.servers[j].ha_status == "ha_real_master") {
                            ha_real_master = j;
                        }
                    }
                    if (ha_real_master >= 0) {
                        confused.push({group: group.id, logical_master: group.servers[0].server, ha_real_master: group.servers[ha_real_master].server});
                    }
                }
                if (confused.length == 0) {
                    alertAction("Reinit and Start proxy: " + toJsonHtml(proxy), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/proxy/reinit/" + xauth + "/" + proxy.token, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(proxy);
                    prompts += "\n\n";
                    prompts += "HA: real master & logical master area conflicting: " + toJsonHtml(confused);
                    prompts += "\n\n";
                    prompts += "Please fix these before resync proxy-[" + proxy.token + "].";
                    alertAction2("Reinit and Start proxy: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/proxy/reinit/" + xauth + "/" + proxy.token, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.createGroup = function (group_id) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(group_id)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/create/" + xauth + "/" + group_id, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.removeGroup = function (group_id) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/remove/" + xauth + "/" + group_id, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.resyncGroup = function (group) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var o = {};
                o.id = group.id;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0) {
                    alertAction("Resync Group-[" + group.id + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync/" + xauth + "/" + group.id, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + group.servers[ha_real_master].server + " should be the real group master, do you really want to resync group-[" + group.id + "] ??";
                    alertAction2("Resync Group-[" + group.id + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync/" + xauth + "/" + group.id, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.resyncGroupAll = function() {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var ha_real_master = -1;
                var gids = [];
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var group = $scope.group_array[i];
                    for (var j = 0; j < group.servers.length; j++) {
                        if (group.servers[j].ha_status == "ha_real_master") {
                            ha_real_master = j;
                        }
                    }
                    gids.push(group.id);
                }
                if (ha_real_master < 0) {
                    alertAction("Resync All Groups: group-[" + gids + "]", function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    alertAction2("Resync All Groups: group-[" + gids + "] (in conflict with HA)", function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.resyncSentinels = function () {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var servers = [];
                for (var i = 0; i < $scope.sentinel_servers.length; i ++) {
                    servers.push($scope.sentinel_servers[i].server);
                }
                var confused = [];
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var group = $scope.group_array[i];
                    var ha_real_master = -1;
                    for (var j = 0; j < group.servers.length; j ++) {
                        if (group.servers[j].ha_status == "ha_real_master") {
                            ha_real_master = j;
                        }
                    }
                    if (ha_real_master >= 0) {
                        confused.push({group: group.id, logical_master: group.servers[0].server, ha_real_master: group.servers[ha_real_master].server});
                    }
                }
                if (confused.length == 0) {
                    alertAction("Resync All Sentinels: " + toJsonHtml(servers), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/sentinels/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(servers);
                    prompts += "\n\n";
                    prompts += "HA: real master & logical master area conflicting: " + toJsonHtml(confused);
                    prompts += "\n\n";
                    prompts += "Please fix these before resync sentinels.";
                    alertAction2("Resync All Sentinels: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/sentinels/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.addSentinel = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/sentinels/add/" + xauth + "/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.delSentinel = function (sentinel, force) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var prefix = "";
                if (force) {
                    prefix = "[FORCE] ";
                }
                alertAction(prefix + "Remove sentinel " + sentinel.server, function () {
                    var xauth = genXAuth(codis_name);
                    var value = 0;
                    if (force) {
                        value = 1;
                    }
                    var url = concatUrl("/api/topom/sentinels/del/" + xauth + "/" + sentinel.server + "/" + value, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }

        $scope.addGroupServer = function (group_id, datacenter, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(group_id) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                if (datacenter == undefined) {
                    datacenter = "";
                } else {
                    datacenter = datacenter.trim();
                }
                var suffix = "";
                if (datacenter != "") {
                    suffix = "/" + datacenter;
                }
                var url = concatUrl("/api/topom/group/add/" + xauth + "/" + group_id + "/" + server_addr + suffix, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.delGroupServer = function (group, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var o = {};
                o.id = group.id;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0 || group.servers[ha_real_master].server != server_addr) {
                    alertAction("Remove server " + server_addr + " from Group-[" + group.id + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/del/" + xauth + "/" + group.id + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + server_addr + " should be the real group master, do you really want to remove it ??";
                    alertAction2("Remove server " + server_addr + " from Group-[" + group.id + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/del/" + xauth + "/" + group.id + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });

                }
            }
        }

        $scope.promoteServer = function (group, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var o = {};
                o.id = group.id;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0 || group.servers[ha_real_master].server == server_addr) {
                    alertAction("Promote server " + server_addr + " from Group-[" + group.id + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/promote/" + xauth + "/" + group.id + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + group.servers[ha_real_master].server + " should be the real group master, do you really want to promote " + server_addr + " ??";
                    alertAction2("Promote server " + server_addr + " from Group-[" + group.id + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/promote/" + xauth + "/" + group.id + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.enableReplicaGroups = function (group_id, server_addr, replica_group) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(group_id) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var value = 0;
                if (replica_group) {
                    value = 1;
                }
                var url = concatUrl("/api/topom/group/replica-groups/" + xauth + "/" + group_id + "/" + server_addr + "/" + value, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.enableReplicaGroupsAll = function (value) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/replica-groups-all/" + xauth + "/" + value, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.createSyncAction = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/action/create/" + xauth + "/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.removeSyncAction = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/action/remove/" + xauth + "/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.createSlotActionSome = function (slots_num, group_from, group_to) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(slots_num) && isValidInput(group_from) && isValidInput(group_to)) {
                alertAction("Migrate " + slots_num + " Slots from Group-[" + group_from + "] to Group-[" + group_to + "]", function () {
                    var xauth = genXAuth(codis_name);
                    var url = concatUrl("/api/topom/slots/action/create-some/" + xauth + "/" + group_from + "/" + group_to + "/" + slots_num, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }

        $scope.createSlotActionRange = function (slot_beg, slot_end, group_id) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(slot_beg) && isValidInput(slot_end) && isValidInput(group_id)) {
                alertAction("Migrate Slots-[" + slot_beg + "," + slot_end + "] to Group-[" + group_id + "]", function () {
                    var xauth = genXAuth(codis_name);
                    var url = concatUrl("/api/topom/slots/action/create-range/" + xauth + "/" + slot_beg + "/" + slot_end + "/" + group_id, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }

        $scope.removeSlotAction = function (slot_id) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(slot_id)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/slots/action/remove/" + xauth + "/" + slot_id, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.updateSlotActionDisabled = function (value) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/slots/action/disabled/" + xauth + "/" + value, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.updateSlotActionInterval = function (value) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/slots/action/interval/" + xauth + "/" + value, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.rebalanceAllSlots = function() {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/slots/rebalance/" + xauth + "/0", codis_name);
                $http.put(url).then(function (resp) {
                    var actions = []
                    for (var i = 0; i < $scope.group_array.length; i ++) {
                        var g = $scope.group_array[i];
                        var slots = [], beg = 0, end = -1;
                        for (var sid = 0; sid < 1024; sid ++) {
                            if (resp.data[sid] == g.id) {
                                if (beg > end) {
                                    beg = sid; end = sid;
                                } else if (end == sid - 1) {
                                    end = sid;
                                } else {
                                    slots.push("[" + beg + "," + end + "]");
                                    beg = sid; end = sid;
                                }
                            }
                        }
                        if (beg <= end) {
                            slots.push("[" + beg + "," + end + "]");
                        }
                        if (slots.length == 0) {
                            continue;
                        }
                        actions.push("group-[" + g.id + "] <== " + slots);
                    }
                    alertAction("Preview of Auto-Rebalance: " + toJsonHtml(actions), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/slots/rebalance/" + xauth + "/1", codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        if (window.location.hash) {
            $scope.selectCodisInstance(window.location.hash.substring(1));
        }

        var ticker = 0;
        (function autoRefreshStats() {
            if (ticker >= $scope.refresh_interval) {
                ticker = 0;
                $scope.refreshStats();
            }
            ticker++;
            $timeout(autoRefreshStats, 1000);
        }());
    }
])
;
