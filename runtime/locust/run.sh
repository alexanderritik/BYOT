#!/bin/sh
# BYOT locust launcher. Runs either a single uploaded locustfile (./artifact)
# or an unzipped bundle with locustfile.py at its root, installing the
# bundle's requirements.txt first. byot_checks.py adds thresholds and keeps
# the run inside the test timeout.
set -e
export BYOT_STARTED_AT="${BYOT_STARTED_AT:-$(date +%s)}"
checks=/app/.byot/byot_checks.py

if [ -f /app/artifact ]; then
	# Locust only loads files ending in .py.
	cp /app/artifact /tmp/locustfile.py
	locustfile=/tmp/locustfile.py
else
	cd /app
	locustfile="${LOCUST_LOCUSTFILE:-locustfile.py}"
	if [ ! -e "$locustfile" ]; then
		echo "[byot] ERROR: bundle has no $locustfile at its root (set LOCUST_LOCUSTFILE to use another file)" >&2
		exit 1
	fi
	if [ -f requirements.txt ]; then
		echo "[byot] installing requirements.txt" >&2
		# --prefix keeps packages already in the image (locust itself) instead
		# of reinstalling them; /tmp is the only writable place.
		pip install --quiet --disable-pip-version-check --no-warn-script-location \
			--prefix /tmp/deps -r requirements.txt
		PYTHONPATH="$(echo /tmp/deps/lib/python*/site-packages)${PYTHONPATH:+:$PYTHONPATH}"
		export PYTHONPATH
	fi
fi

# 5 users at 1/s unless the env or the bundle's locust.conf says otherwise.
if [ ! -f locust.conf ]; then
	export LOCUST_USERS="${LOCUST_USERS:-5}"
	export LOCUST_SPAWN_RATE="${LOCUST_SPAWN_RATE:-1}"
fi

exec locust -f "$locustfile,$checks" --headless --only-summary "$@"
