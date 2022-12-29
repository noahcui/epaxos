#!/usr/bin/env python3
import psutil
import time

for i in range(80):
    time.sleep(1)
    cpu = psutil.cpu_percent()
    mem = psutil.virtual_memory().used
    print("%s,%s" % (cpu, mem/(1024*1024)))
