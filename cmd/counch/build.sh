#!/bin/bash
rm -f counch.x
go build -o counch.x main.go
cp counch.x ../../test_node1
cp counch.x ../../test_node2