using System;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Logging;
using cartservice.cartstore;
using Msdemo;
using System.Threading;

namespace cartservice.services
{
    public class CartService : Msdemo.CartService.CartServiceBase
    {
        private readonly static Empty Empty = new Empty();
        private readonly ICartStore _cartStore;

        public CartService(ICartStore cartStore)
        {
            _cartStore = cartStore;
        }

        public async override Task<Empty> AddItem(AddItemRequest request, ServerCallContext context)
        {
            CartService.SleepForSomeTime();
            await _cartStore.AddItemAsync(request.UserId, request.Item.ProductId, request.Item.Quantity);
            return Empty;
        }

        public override Task<Cart> GetCart(GetCartRequest request, ServerCallContext context)
        {
            CartService.SleepForSomeTime();
            return _cartStore.GetCartAsync(request.UserId);
        }

        public async override Task<Empty> EmptyCart(EmptyCartRequest request, ServerCallContext context)
        {
            CartService.SleepForSomeTime();
            await _cartStore.EmptyCartAsync(request.UserId);
            return Empty;
        }

        public static void SleepForSomeTime()
        {
            Random rnd = new Random();
            int sleepWaitMilliSeconds = rnd.Next(25, 250);
            Thread.Sleep(sleepWaitMilliSeconds);
        }
    }
}