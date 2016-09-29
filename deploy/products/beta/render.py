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
    content = "#!/usr/bin/env bash\n\n" + content
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
            'BIN_PATH': self.env.bin_path,
            'ETC_PATH': self.env.etc_path,
            'LOG_PATH': self.env.log_path,
        }
        base = os.path.join(generate_root, self.env.etc_path.lstrip('/'), self.admin_addr)

        temp = readfile(template_root, "dashboard.toml.template")
        generate(base, "dashboard.toml", temp.format(**kwargs))

        temp = readfile(template_root, "dashboard.service.template")
        generate(base, "codis_dashboard_{}.service".format(self.admin_port), temp.format(**kwargs))

        admin = os.path.join(self.env.bin_path, "codis-admin")
        generate_bash(base, "dashboard_admin.sh", "{} --dashboard={} $@".format(admin, self.admin_addr))

        scripts = ''
        for p in proxylist:
            scripts += "{} --dashboard={} --online-proxy --addr={}".format(admin, self.admin_addr, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "online_proxy.sh", scripts)

        scripts = ''
        for p in proxylist:
            scripts += "{} --dashboard={} --reinit-proxy --addr={}".format(admin, self.admin_addr, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "reinit_proxy.sh", scripts)

        scripts = ''
        for p in proxylist:
            scripts += "{} --proxy={} $@".format(admin, p.admin_addr)
            scripts += "\n"
        generate_bash(base, "foreach_proxy.sh", scripts)


class Template:
    def __init__(self, config):
        self.ncpu = config.get("ncpu", 4)
        self.max_pipeline = config.get("max_pipeline", 1024)
        self.jodis_addr = config.get("jodis_addr", "")


class Proxy():
    def __init__(self, product, template, config):
        self.product = product
        self.env = product.env
        self.template = template
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
            'MAX_PIPELINE': self.template.max_pipeline,
            'JODIS_ADDR': self.template.jodis_addr,
            'NCPU': self.template.ncpu,
            'BIN_PATH': self.env.bin_path,
            'ETC_PATH': self.env.etc_path,
            'LOG_PATH': self.env.log_path,
            'LOG_LEVEL': self.env.log_level,
        }
        base = os.path.join(generate_root, self.env.etc_path.lstrip('/'), self.proxy_addr)

        temp = readfile(template_root, "proxy.toml.template")
        generate(base, "proxy.toml", temp.format(**kwargs))

        temp = readfile(template_root, "proxy.service.template")
        generate(base, "codis_proxy_{}.service".format(self.proxy_port), temp.format(**kwargs))

        admin = os.path.join(self.env.bin_path, "codis-admin")
        generate_bash(base, "proxy_admin.sh", "{} --proxy={} $@".format(admin, self.admin_addr))


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
            template = Template(config["proxy"].get("template", {}))
            for p in config["proxy"].get("instance", []):
                self.proxylist.append(Proxy(self, template, p))
        self.proxylist.sort(key=lambda p: p.proxy_addr)

    def render(self):
        self.dashboard.render(self.proxylist)
        for p in self.proxylist:
            p.render()


config = {}

with open('config.json') as f:
    config = json.loads(f.read())

product = Product(config)
product.render()
