#!/usr/bin/env python3

import os
import json
import stat

template_root = "../../templates"
generate_root = "../../root"

def readfile(path, name):
    file = os.path.join(path, name)
    with open(file) as f:
        return f.read()


def generate(path, name, content):
    if path != "":
        os.makedirs(path, exist_ok=True)
    file = os.path.join(path, name)
    print("Generate: {}".format(os.path.abspath(file)))
    with open(file, 'w+') as f:
        f.write(content)


def generate_bash(path, name, content):
    content = "#!/usr/bin/env bash\n\nset -x\nset -e\n\n" + content + "\n"
    generate(path, name, content)
    file = os.path.join(path, name)
    os.chmod(file, os.stat(file).st_mode | stat.S_IEXEC)


def symlink(src, dst):
    try:
        os.remove(dst)
    except OSError:
        pass
    os.symlink(src, dst)


class Coordinator:
    def __init__(self, config):
        self.name = config.get("name", "")
        self.addr = config.get("addr", "")


class Dashboard():
    def __init__(self, product, config):
        self.product = product
        self.env = product.env
        self.admin_addr = config.get("admin_addr", "")

        self.sentinel_quorum = config.get("sentinel_quorum", 2)
        self.sentinel_down_after = config.get("sentinel_down_after", "30s")
        self.coordinator = Coordinator(config.get("coordinator", {}))

        if self.admin_addr == "":
            raise Exception("dashboard.admin_addr not found")
        if self.coordinator.name == "":
            raise Exception("coordinator.name not found")
        if self.coordinator.addr == "":
            raise Exception("coordinator.addr not found or empty")

        self.admin_port = self.admin_addr.rsplit(':', 1)[1]

    def render(self, proxylist):
        kwargs = {
            'PRODUCT_NAME': self.product.name,
            'PRODUCT_AUTH': self.product.auth,
            'COORDINATOR_NAME': self.coordinator.name,
            'COORDINATOR_ADDR': self.coordinator.addr,
            'ADMIN_ADDR': self.admin_addr,
            'ADMIN_PORT': self.admin_port,
            'SENTINEL_QUORUM': self.sentinel_quorum,
            'SENTINEL_DOWN_AFTER': self.sentinel_down_after,
            'BIN_PATH': self.env.bin_path,
            'ETC_PATH': self.env.etc_path,
            'LOG_PATH': self.env.log_path,
        }
        base = os.path.join(generate_root, self.env.etc_path.lstrip('/'), self.admin_addr.replace(':', '_'))

        temp = readfile(template_root, "dashboard.toml.template")
        generate(base, "dashboard.toml", temp.format(**kwargs))

        temp = readfile(template_root, "dashboard.service.template")
        generate(base, "codis_dashboard_{}.service".format(self.admin_port), temp.format(**kwargs))

        admin = os.path.join(self.env.bin_path, "codis-admin")
        generate_bash(base, "dashboard_admin", "{} --dashboard={} $@".format(admin, self.admin_addr))

        scripts = 'd=1\n'
        for p in proxylist:
            scripts += "sleep $d; {} --dashboard={} --online-proxy --addr={}".format(admin, self.admin_addr, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "foreach_proxy_online", scripts)

        scripts = 'd=1\n'
        for p in proxylist:
            scripts += "sleep $d; {} --dashboard={} --reinit-proxy --addr={}".format(admin, self.admin_addr, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "foreach_proxy_reinit", scripts)

        scripts = 'd=1\n'
        for p in proxylist:
            scripts += "sleep $d; {} --proxy={} $@".format(admin, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "foreach_proxy", scripts)

        cwd = os.getcwd()
        os.chdir(os.path.join(base, ".."))
        symlink(self.admin_addr.replace(':', '_'), self.product.name)
        os.chdir(cwd)


class Template:
    def __init__(self, config):
        self.min_cpu = config.get("min_cpu", 4)
        self.max_cpu = config.get("max_cpu", 0)
        self.max_clients = config.get("max_clients", 10000)
        self.max_pipeline = config.get("max_pipeline", 1024)
        self.log_level = config.get("log_level", "INFO")
        self.jodis_name = config.get("jodis_name", "")
        self.jodis_addr = config.get("jodis_addr", "")


class Proxy():
    def __init__(self, product, template, config):
        self.product = product
        self.env = product.env
        self.template = template
        self.datacenter = config.get("datacenter", "")
        self.admin_addr = config.get("admin_addr", "")
        self.proxy_addr = config.get("proxy_addr", "")

        if self.admin_addr == "":
            raise Exception("proxy.admin_addr not found")
        if self.proxy_addr == "":
            raise Exception("proxy.proxy_addr not found")

        self.proxy_port = self.proxy_addr.rsplit(':', 1)[1]

    def render(self):
        kwargs = {
            'PRODUCT_NAME': self.product.name,
            'PRODUCT_AUTH': self.product.auth,
            'ADMIN_ADDR': self.admin_addr,
            'PROXY_ADDR': self.proxy_addr,
            'PROXY_PORT': self.proxy_port,
            'DATACENTER': self.datacenter,
            'MAX_CLIENTS': self.template.max_clients,
            'MAX_PIPELINE': self.template.max_pipeline,
            'JODIS_NAME': self.template.jodis_name,
            'JODIS_ADDR': self.template.jodis_addr,
            'MIN_CPU': self.template.min_cpu,
            'MAX_CPU': self.template.max_cpu,
            'LOG_LEVEL': self.template.log_level,
            'BIN_PATH': self.env.bin_path,
            'ETC_PATH': self.env.etc_path,
            'LOG_PATH': self.env.log_path,
        }
        base = os.path.join(generate_root, self.env.etc_path.lstrip('/'), self.proxy_addr.replace(':', '_'))

        temp = readfile(template_root, "proxy.toml.template")
        generate(base, "proxy.toml", temp.format(**kwargs))

        temp = readfile(template_root, "proxy.service.template")
        generate(base, "codis_proxy_{}.service".format(self.proxy_port), temp.format(**kwargs))

        admin = os.path.join(self.env.bin_path, "codis-admin")
        generate_bash(base, "proxy_admin", "{} --proxy={} $@".format(admin, self.admin_addr))


class Env:
    def __init__(self, config):
        self.bin_path = os.path.abspath(config.get("bin_path", "/opt/codis/bin"))
        self.etc_path = os.path.abspath(config.get("etc_path", "/opt/codis/etc"))
        self.log_path = os.path.abspath(config.get("log_path", "/opt/codis/log"))
        self.log_level = config.get("log_level", "INFO").upper()


class Product:
    def __init__(self, config):
        self.name = config.get("product_name", "")
        self.auth = config.get("product_auth", "")

        if self.name == "":
            raise Exception("product_name not found")

        self.env = Env(config.get("env", {}))
        self.dashboard = Dashboard(self, config.get("dashboard", {}))

        self.proxylist = []
        if "proxy" in config:
            template = Template(config.get("proxy", {}))
            for p in config.get("instance", []):
                self.proxylist.append(Proxy(self, template, p))
        self.proxylist.sort(key=lambda p: p.datacenter + "|" + p.proxy_addr)

    def render(self):
        self.dashboard.render(self.proxylist)
        for p in self.proxylist:
            p.render()


config = {}

with open('config.json') as f:
    config = json.loads(f.read())

with open('instance.json') as f:
    config["instance"] = json.loads(f.read())

product = Product(config)
product.render()
