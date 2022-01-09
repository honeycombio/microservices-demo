'use strict';

const path = require('path');
const microservicesDemoServer = require('./server');

const PORT = process.env['PORT'];
const PROTO_PATH = path.join(__dirname, '/proto/');

const server = new microservicesDemoServer(PROTO_PATH, PORT);

server.listen();
