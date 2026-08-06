#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

  if [ "${ENTRYPOINT_COMMAND_EXECUTED:-N}" == "N" ]; then
    b19-log info "BRIDGE" "$(_ "Starting")"

    sleep infinity
    # b19-exec -- pf-bridge "$@"
  fi
