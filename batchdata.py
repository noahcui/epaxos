#!/usr/bin/env python3

from collections import namedtuple
from locale import atof
from unicodedata import name
from unittest import result
import matplotlib
import matplotlib.pyplot as plt
import numpy as np
import os
import sys
from datetime import datetime
import argparse
parser = argparse.ArgumentParser(description='Process some integers.')

parser.add_argument('-d', type=str)
# for filename in os.listdir(dir):
args = parser.parse_args()


class OverAllResult:
    def __init__(self):
        self.Concurrency = 0.0
        self.BenchTime = "0s"
        self.x = 0.0
        self.p50 = 0.0
        self.p95 = 0.0
        self.p99 = 0.0
        self.p999 = 0.0
        self.pmax = 0.0
        self.pmin = 0.0
        self.pavg = 0.0

    def printall(self):
        attrs = vars(self)
        print(', '.join("%s: %s" % item for item in attrs.items()))


def benchlogfileresolver(dir, file):
    f = open(dir+file, "r")
    to_return = OverAllResult()
    for line in reversed(f.readlines()):
        if line.find("BenchmarkTime = ") > -1:
            to_return.BenchTime = line.split(" ")[-1]
        if line.find("Concurrency = ") > -1:
            to_return.Concurrency = float(line.split(" ")[-1])
        if line.find("overall50 = ") > -1:
            to_return.p50 = float(line.split(" ")[-1])
        if line.find("overall95 = ") > -1:
            to_return.p95 = float(line.split(" ")[-1])
        if line.find("overall99 = ") > -1:
            to_return.p99 = float(line.split(" ")[-1])
        if line.find("overall999 = ") > -1:
            to_return.p999 = float(line.split(" ")[-1])
        if line.find("overallmax = ") > -1:
            to_return.pmax = float(line.split(" ")[-1])
        if line.find("overallmin = ") > -1:
            to_return.pmin = float(line.split(" ")[-1])
        if line.find("overallavg = ") > -1:
            to_return.pavg = float(line.split(" ")[-1])
        if line.find("overallx = ") > -1:
            to_return.x = float(line.split(" ")[-1])
    f.close()
    return to_return


def sort_on_Concurrency(to_sort):
    n = len(to_sort)
    for i in range(n):
        for j in range(0, n-i-1):
            if to_sort[j].Concurrency > to_sort[j+1].Concurrency:
                to_sort[j], to_sort[j+1] = to_sort[j+1], to_sort[j]
    return to_sort


def solvebenchlog(dir):
    results = []
    for fname in os.listdir(dir):
        to_append = benchlogfileresolver(dir, fname)
        results.append(to_append)
    sorted = sort_on_Concurrency(results)
    fo = open(dir+"results.csv", "w+")
    fo.write("clients,x,p50,p95,p99,p999,max,min,avg\n")
    for i in range(len(sorted)):
        item = sorted[i]
        mark = ","
        fo.write("%s%s" % (item.Concurrency, mark))
        fo.write("%s%s" % (item.x, mark))
        fo.write("%s%s" % (item.p50, mark))
        fo.write("%s%s" % (item.p95, mark))
        fo.write("%s%s" % (item.p99, mark))
        fo.write("%s%s" % (item.p999, mark))
        fo.write("%s%s" % (item.pmax, mark))
        fo.write("%s%s" % (item.pmin, mark))
        fo.write("%s%s" % (item.pavg, mark))
        fo.write("\n")
    fo.close()


if __name__ == "__main__":

    # drawlatency(sys.argv[1])
    solvebenchlog(args.d)
