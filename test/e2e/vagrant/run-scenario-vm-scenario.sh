#!/usr/bin/env bash

apply_offline_guard() {
  local guard_log="${CASE_DIR}/03-offline-guard.log"

  echo "OFFLINE_GUARD stage=start" | tee "${guard_log}"
  sudo -n iptables -N DECK_OFFLINE_GUARD >/dev/null 2>&1 || true
  sudo -n iptables -F DECK_OFFLINE_GUARD
  if ! sudo -n iptables -C OUTPUT -j DECK_OFFLINE_GUARD >/dev/null 2>&1; then
    sudo -n iptables -I OUTPUT 1 -j DECK_OFFLINE_GUARD
  fi
  sudo -n iptables -A DECK_OFFLINE_GUARD -o lo -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -d 127.0.0.0/8 -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -d 10.0.0.0/8 -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -d 172.16.0.0/12 -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -d 192.168.0.0/16 -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  sudo -n iptables -A DECK_OFFLINE_GUARD -j REJECT

  if command -v ip6tables >/dev/null 2>&1; then
    sudo -n ip6tables -N DECK_OFFLINE_GUARD6 >/dev/null 2>&1 || true
    sudo -n ip6tables -F DECK_OFFLINE_GUARD6
    if ! sudo -n ip6tables -C OUTPUT -j DECK_OFFLINE_GUARD6 >/dev/null 2>&1; then
      sudo -n ip6tables -I OUTPUT 1 -j DECK_OFFLINE_GUARD6
    fi
    sudo -n ip6tables -A DECK_OFFLINE_GUARD6 -o lo -j ACCEPT
    sudo -n ip6tables -A DECK_OFFLINE_GUARD6 -d ::1/128 -j ACCEPT
    sudo -n ip6tables -A DECK_OFFLINE_GUARD6 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
    sudo -n ip6tables -A DECK_OFFLINE_GUARD6 -j REJECT
  fi
  OFFLINE_GUARD_ACTIVE=1

  if timeout 5 curl -fsS https://deb.debian.org >/dev/null 2>&1; then
    echo "OFFLINE_GUARD egress=FAILED" | tee -a "${guard_log}"
    return 1
  fi

  if ! curl -fsS --max-time 5 "${SERVER_URL}/healthz" >/dev/null 2>&1; then
    echo "OFFLINE_GUARD local_server=FAILED" | tee -a "${guard_log}"
    return 1
  fi

  echo "OFFLINE_GUARD egress=BLOCKED local_server=OK" | tee -a "${guard_log}"
  return 0
}
