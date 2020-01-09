#!/bin/env python
import sys
with open(sys.argv[1],"r") as r,open(sys.argv[2],"w") as w:
    w.write(r.read())
