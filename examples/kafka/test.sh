cat ../../route/fixtures/metrics.txt| docker exec -i kafka_kafka_1 kafka-console-producer --broker-list localhost:9092 --topic metrics
sleep 10
cat /tmp/metrics-test-output-nonsorted.txt | sort > /tmp/metrics-test-output.txt
echo "nb line $(wc -l metrics-test-input.txt)"
echo "nb line $(wc -l /tmp/metrics-test-output.txt)"
echo DIFF:
diff -q metrics-test-input.txt /tmp/metrics-test-output.txt
