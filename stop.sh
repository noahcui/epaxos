PID_FILE=$1
if [ ! -f "${PID_FILE}" ]; then
    echo "No server is running1."
else
	while read pid; do
		if [ -z "${pid}" ]; then
			echo "No server is running2."
		else
			sudo kill -15 "${pid}"
			echo "Server with PID ${pid} shutdown."
    	fi
	done < "${PID_FILE}"
	rm "${PID_FILE}"
fi