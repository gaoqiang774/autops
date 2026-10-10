const os = require('os')
const path = require('path')
const net = require('net')
const { spawn } = require('child_process')

const requestedPort = Number(process.env.PORT || 8088)
const appBasePath = '/'

const localIPv4Addresses = Object.values(os.networkInterfaces())
  .flat()
  .filter(detail => {
    const family = typeof detail?.family === 'string' ? detail.family : String(detail?.family)
    return detail &&
      family === 'IPv4' &&
      !detail.internal &&
      !detail.address.startsWith('169.254.')
  })
  .map(detail => detail.address)
  .filter((address, index, addresses) => addresses.indexOf(address) === index)

const terminalLink = url => {
  return `\u001b]8;;${url}\u0007${url}\u001b]8;;\u0007`
}

const buildAppUrl = (host, port) => {
  return `http://${host}:${port}${appBasePath}`
}

const isPortAvailable = port => {
  return new Promise(resolve => {
    const server = net.createServer()
    server.once('error', () => resolve(false))
    server.once('listening', () => {
      server.close(() => resolve(true))
    })
    server.listen(port, '0.0.0.0')
  })
}

const findAvailablePort = async port => {
  let currentPort = port
  while (!(await isPortAvailable(currentPort))) {
    currentPort += 1
  }
  return currentPort
}

const start = async () => {
  const port = requestedPort === null ? null : await findAvailablePort(requestedPort)
  let appendedNetworkAddress = false
  const nodeOptions = [process.env.NODE_OPTIONS, '--no-deprecation']
    .filter(Boolean)
    .join(' ')

  const printNetworkAddress = prefix => {
    if (!localIPv4Addresses.length) {
      console.log(`${prefix}- Network IPv4: unavailable`)
      return
    }

    localIPv4Addresses.forEach(address => {
      const url = buildAppUrl(address, port)
      console.log(`${prefix}- Network IPv4: ${terminalLink(url)}`)
    })
  }

  console.log('')
  console.log('  Dev server addresses:')
  if (port !== null) {
    console.log(`  - Local:        ${terminalLink(buildAppUrl('localhost', port))}`)
    printNetworkAddress('  ')
  }

  if (requestedPort !== null && port !== requestedPort) {
    console.log(`  - Note: requested port ${requestedPort} is busy, using ${port}`)
  }

  console.log('')

  const command = process.execPath
  const args = [
    path.resolve(__dirname, '../node_modules/@vue/cli-service/bin/vue-cli-service.js'),
    'serve',
    '--mode',
    'dev',
    '--host',
    '0.0.0.0'
  ]

  if (port !== null) {
    args.push('--port', String(port))
  }

  const child = spawn(command, args, {
    stdio: ['inherit', 'pipe', 'pipe'],
    shell: false,
    env: {
      ...process.env,
      CHOKIDAR_USEPOLLING: 'true',
      NODE_OPTIONS: nodeOptions,
      WATCHPACK_POLLING: 'true'
    }
  })

  const outputPorts = []

  child.stdout.on('data', chunk => {
    let text = chunk.toString()
    if (localIPv4Addresses.length && port !== null) {
      const networkUrl = buildAppUrl(localIPv4Addresses[0], port)
      text = text.replace(/- Network:\s+unavailable/g, `- Network:   ${networkUrl}`)
    }
    process.stdout.write(text)
    const matches = text.matchAll(/https?:\/\/[^\s/:]+:([0-9]+)/g)
    for (const match of matches) {
      const outputPort = Number(match[1])
      if (!outputPorts.includes(outputPort)) outputPorts.push(outputPort)
    }
  })

  child.stderr.on('data', chunk => {
    process.stderr.write(chunk)
  })

  child.on('exit', code => {
    process.exit(code ?? 0)
  })

}

start().catch(error => {
  console.error(error)
  process.exit(1)
})
