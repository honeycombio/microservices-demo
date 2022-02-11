const path = require('path');
const grpc = require('@grpc/grpc-js');
const pino = require('pino');
const protoLoader = require('@grpc/proto-loader');
const opentelemetry = require('@opentelemetry/api');

const charge = require('./charge');

const logger = pino({
    name: 'paymentservice-server',
    messageKey: 'message',
    changeLevelName: 'severity',
    useLevelLabels: true
});

function getRandomWaitTime(max, buckets) {
    let num = 0;
    const val = max / buckets;
    for (let i = 0; i < buckets; i++) {
        num += Math.random() * val;
    }
    return num;
}

function sleepRandom(max) {
    const rnd = getRandomWaitTime(max, 4);
    return new Promise((resolve, reject) => {
        setTimeout(resolve, rnd);
    })
}

async function mockDatabaseCall(maxTime, name, query) {
    const tracer = opentelemetry.trace.getTracer("");
    const span = tracer.startSpan(name);
    span.setAttribute("db.statement", query);
    span.setAttribute("db.name", "payment");
    await sleepRandom(maxTime);
    span.end();
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
        await sleepRandom(75);
        await mockDatabaseCall(30, "INSERT payment.charges", "INSERT INTO charges (amount, card_hash, card_type, last_4) VALUES (?, ?, ?, ?)");
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
