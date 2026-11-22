#!/bin/sh
# Build the Pagefind search index (and its UI files) into a built site:
#   scripts/site_search_index.sh DIR
# Pagefind runs on the static output after hugo, entirely offline; the version
# is pinned so the index and UI never change under us.
set -eu
PAGEFIND_VERSION="1.5.2"
site="${1:?usage: site_search_index.sh BUILT_SITE_DIR}"
npx --yes "pagefind@${PAGEFIND_VERSION}" --site "$site" --quiet
