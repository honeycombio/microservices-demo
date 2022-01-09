// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const path = require('path');
const grpc = require('@grpc/grpc-js');
const pino = require('pino');
const protoLoader = require('@grpc/proto-loader');

const charge = require('./charge');

const logger = pino({
    name: 'paymentservice-server',
    messageKey: 'message',
    changeLevelName: 'severity',
    useLevelLabels: true
});

function sleep(wait_time) {
    // mock some work by sleeping
    return new Promise((resolve, reject) => {
        setTimeout(resolve, wait_time);
    })
}

function getRandomInt(max) {
    return Math.floor(Math.random() * max) + 1;
}


class MicroservicesDemoServer {
    constructor(protoRoot, port = MicroservicesDemoServer.PORT) {
        this.port = port;

        this.packages = {
            msdemo: this.loadProto(path.join(protoRoot, 'demo.proto')),
            health: this.loadProto(path.join(protoRoot, 'grpc/health/v1/health.proto'))
        };

        this.server = new grpc.Server();
        this.loadAllProtos(protoRoot);
    }

    /**
     * Handler for PaymentService.Charge.
     * @param {*} call  { ChargeRequest }
     * @param {*} callback  fn(err, ChargeResponse)
     */
    static async ChargeServiceHandler(call, callback) {
        await sleep(getRandomInt(100));
        try {
            logger.info(`PaymentService#Charge invoked with request ${JSON.stringify(call.request)}`);
            const response = charge(call.request);
            callback(null, response);
        } catch (err) {
            console.warn(err);
            callback(err);
        }
    }

    static CheckHandler(call, callback) {
        callback(null, {status: 'SERVING'});
    }


    listen() {
        const server = this.server
        const port = this.port
        server.bindAsync(
            `0.0.0.0:${port}`,
            grpc.ServerCredentials.createInsecure(),
            function () {
                logger.info(`PaymentService gRPC server started on port ${port}`);
                server.start();
            }
        );
    }

    loadProto(path) {
        const packageDefinition = protoLoader.loadSync(
            path,
            {
                keepCase: true,
                longs: String,
                enums: String,
                defaults: true,
                oneofs: true
            }
        );
        return grpc.loadPackageDefinition(packageDefinition);
    }

    loadAllProtos(protoRoot) {
        const msdemoPackage = this.packages.msdemo.msdemo;
        const healthPackage = this.packages.health.grpc.health.v1;

        this.server.addService(
            msdemoPackage.PaymentService.service,
            {
                charge: MicroservicesDemoServer.ChargeServiceHandler.bind(this)
            }
        );

        this.server.addService(
            healthPackage.Health.service,
            {
                check: MicroservicesDemoServer.CheckHandler.bind(this)
            }
        );
    }
}

MicroservicesDemoServer.PORT = process.env.PORT;

module.exports = MicroservicesDemoServer;
