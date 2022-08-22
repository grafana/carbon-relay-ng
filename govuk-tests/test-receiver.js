/**
 * This script creates a basic TCP client and server. The server mimics the
 * role that Stunnel performs in our environments and accepts connections from
 * carbon-relay and reads the formatted metrics it sends.
 *
 * The client is used to send a metric to carbon-relay to begin the process.
 * Carbon-relay should correctly format it and then send it to the server.
 *
 * The sequence of events is:
 * 1. Create a server and listen on 0.0.0.0:20030
 * 2. Accept a connection from carbon-relay
 * 3. Send a metric to carbon-relay using the client
 * 4. Read the formatted metric from carbon-relay
 * 5. Compare the received metric to the expected value and pass or fail as
 * necessary.
 * 6. If errors are encountered with either connection or carbon-relay closes
 * the connection before a metric is received and checked then exit with a
 * failure.
 **/

const net = require('net')

const RECEIVER_PORT = 20030
const EXPECTED_METRIC = 'api-key.account-name.env-name.metric-name 1234 1637679546'

const client = new net.Socket()

client.on('close', () => {
  console.log('Connection to carbon-relay closed')
})

client.on('error', (error) => {
  console.error(`Error connecting to carbon-relay: ${error}`)
  process.exit(1)
})

const server = net.createServer()

function handleConnection (conn) {
  console.log(`Accepted connection from ${conn.remoteAddress}:${conn.remotePort}`)
  conn.on('data', (chunk) => {
    const receivedMetric = chunk.toString().trim()
    console.log(`Recieved "${receivedMetric}"`)
    if (receivedMetric === EXPECTED_METRIC) {
      console.log('Pass')
      process.exit(0)
    } else {
      console.error(`Expected "${EXPECTED_METRIC}"`)
      process.exit(1)
    }
  })

  conn.on('close', () => {
    console.error('Connection with test server has been closed')
    process.exit(1)
  })

  conn.on('error', (error) => {
    console.error(`Error with connection: ${error}`)
    process.exit(1)
  })

  client.connect(2003, 'carbon-relay', () => {
    console.log('Test client has connected to carbon-relay')
    console.log('Test client Sending metric to carbon-relay')
    client.write('metric-name 1234 1637679546', () => {
      console.log('Finished sending metric')
      client.destroy()
    })
  })
}

server.on('connection', handleConnection)
server.listen(RECEIVER_PORT, '0.0.0.0', () => {
  console.log(`TCP server listening on 0.0.0.0:${RECEIVER_PORT}`)
})
