#!/bin/bash
rm -f counch.x
go build -v -o counch.x main.go
cp counch.x ../../test_node1
cp counch.x ../../test_node2