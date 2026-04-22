#!/bin/bash
systemctl daemon-reload
systemctl enable pfx2as-neighbor
systemctl start pfx2as-neighbor
