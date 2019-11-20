#!/bin/bash
echo 'blabla.test 1 3 {"application_name":"mywonderfullapp"}'| docker exec -i carbon-kafka-input-output_kafka-producer_1 nc localhost 4000
echo 'blabla.test 1 4'| docker exec -i carbon-kafka-input-output_kafka-producer_1 nc localhost 4000
