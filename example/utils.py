#!/usr/bin/env python3

import subprocess
import time
import os


class Process:

    def __init__(self, command, logfile=None):
        self.command = command
        if logfile is not None:
            self.logfile = open(logfile, "wb+")
        try:
            self.proc = subprocess.Popen(self.command.split(), stderr=subprocess.STDOUT, stdout=self.logfile)
        except Exception:
            print("run command failed: {}".format(self.command))
            raise

    def is_running(self):
        try:
            self.proc.wait(0)
        except Exception:
            pass
        return self.proc.returncode is None

    def kill(self):
        if self.is_running():
            self.proc.kill()
        if self.logfile is not None:
            self.logfile.close()

    def wait(self):
        self.proc.wait()

    def get_pid(self):
        return self.proc.pid


def kill_all(children=[]):
    for p in children:
        p.kill()


def check_alive(children=[], seconds=0):
    if seconds != 0:
        time.sleep(seconds)
    for p in children:
        if not p.is_running():
            message = "process lost - {}".format(p.command)
            raise Exception(message)


def do_command(command):
    return subprocess.call(command.split())


if __name__ != "__main__":
    os.environ["PATH"] += os.pathsep + os.getcwd()
    os.environ["PATH"] += os.pathsep + os.path.abspath(os.path.join(os.getcwd(), "../bin"))

