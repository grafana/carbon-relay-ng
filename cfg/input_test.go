package cfg

// const ValidKafka = `
// [[inputs]]
// type = "kafka"
// format = "plain"
// client_id = "123"
// brokers = ["127.0.0.1:1200"]
// topic = "abc"
// `

// // Yes it's a bad test
// // TODO: Add more relevant testing about the config
// func TestValidKafkaConfig(t *testing.T) {
// 	c := NewConfig()
// 	toml.Decode(ValidKafka, &c)
// 	assert.NoError(t, c.ProcessInputConfig())
// 	assert.NotEmpty(t, c.Inputs)
// }
