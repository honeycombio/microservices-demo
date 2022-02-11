using System;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using cartservice.cartstore;
using cartservice.services;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;
using StackExchange.Redis;
using System.Collections.Generic;
namespace cartservice
{
    public class Startup
    {
        public Startup(IConfiguration configuration)
        {
            Configuration = configuration;
        }

        public IConfiguration Configuration { get; }

        // This method gets called by the runtime. Use this method to add services to the container.
        // For more information on how to configure your application, visit https://go.microsoft.com/fwlink/?LinkID=398940
        public void ConfigureServices(IServiceCollection services)
        {

            string redisAddress = Configuration["REDIS_ADDR"];
            RedisCartStore cartStore = null;
            cartStore = new RedisCartStore(redisAddress);
 
            // Initialize the redis store
            cartStore.InitializeAsync().GetAwaiter().GetResult();
            Console.WriteLine("Initialization completed");

            ConnectionMultiplexer redis = cartStore.GetRedisMultiplexer();

            // Get the Service name and OTLP endpoint that will be used for OpenTelemetry
            string servicename = Environment.GetEnvironmentVariable("SERVICE_NAME");
            string otlpendpoint = Environment.GetEnvironmentVariable("OTEL_EXPORTER_OTLP_ENDPOINT");
            if(servicename == null || otlpendpoint == null ) {
                Console.WriteLine("Enviornment variables missing or empty.");
            } else {
                Console.WriteLine("Starting up the open telemetry service");
                AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);

                string podip = Environment.GetEnvironmentVariable("POD_IP");
                IEnumerable<KeyValuePair<string, object>> attributes = new Dictionary<string,object> { {"ip", podip}};

                // Initialize the OpenTelemetry tracing API, with resource attributes
                // setup instrumentation for AspNetCore and HttpClient
                // use the OTLP exporter
                services.AddOpenTelemetryTracing((builder) => builder
                    .SetResourceBuilder(ResourceBuilder.CreateDefault().AddService(servicename).AddAttributes(attributes))
                    .AddAspNetCoreInstrumentation()
                    .AddHttpClientInstrumentation()
                    .AddRedisInstrumentation(redis)
                    .AddOtlpExporter(otlpOptions =>
                    {
                        otlpOptions.Endpoint = new Uri(otlpendpoint);
                    }));
            }

            services.AddSingleton<ICartStore>(cartStore);

            services.AddGrpc();
        }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            if (env.IsDevelopment())
            {
                app.UseDeveloperExceptionPage();
            }

            app.UseRouting();

            app.UseEndpoints(endpoints =>
            {
                endpoints.MapGrpcService<CartService>();
                endpoints.MapGrpcService<cartservice.services.HealthCheckService>();

                endpoints.MapGet("/", async context =>
                {
                    await context.Response.WriteAsync("Communication with gRPC endpoints must be made through a gRPC client. To learn how to create a client, visit: https://go.microsoft.com/fwlink/?linkid=2086909");
                });
            });
        }
    }
}