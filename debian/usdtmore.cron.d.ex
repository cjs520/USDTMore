#
# Regular cron jobs for the usdtmore package.
#
0 4	* * *	root	[ -x /usr/bin/usdtmore_maintenance ] && /usr/bin/usdtmore_maintenance
