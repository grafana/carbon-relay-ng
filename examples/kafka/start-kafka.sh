docker-compose up -d
sleep 10
docker exec kafka_kafka_1 kafka-topics --create --zookeeper kafka_zookeeper_1:2181 --replication-factor 1 --partitions 5 --topic metrics
