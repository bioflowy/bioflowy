#!/bin/env python
import sys


with open(sys.argv[1],'w') as f:
    for i in range(100):
        f.write("test write times:{}\n".format(i))
