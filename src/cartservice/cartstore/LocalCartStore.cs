using System;
using System.Collections.Concurrent;
using System.Threading.Tasks;
using System.Linq;

namespace cartservice.cartstore
{
    internal class LocalCartStore : ICartStore
    {
        // Maps between user and their cart
        private ConcurrentDictionary<string, Msdemo.Cart> userCartItems = new ConcurrentDictionary<string, Msdemo.Cart>();
        private readonly Msdemo.Cart emptyCart = new Msdemo.Cart();

        public Task InitializeAsync()
        {
            Console.WriteLine("Local Cart Store was initialized");

            return Task.CompletedTask;
        }

        public Task AddItemAsync(string userId, string productId, int quantity)
        {
            Console.WriteLine($"AddItemAsync called with userId={userId}, productId={productId}, quantity={quantity}");
            var newCart = new Msdemo.Cart
                {
                    UserId = userId,
                    Items = { new Msdemo.CartItem { ProductId = productId, Quantity = quantity } }
                };
            userCartItems.AddOrUpdate(userId, newCart,
            (k, exVal) =>
            {
                // If the item exists, we update its quantity
                var existingItem = exVal.Items.SingleOrDefault(item => item.ProductId == productId);
                if (existingItem != null)
                {
                    existingItem.Quantity += quantity;
                }
                else
                {
                    exVal.Items.Add(new Msdemo.CartItem { ProductId = productId, Quantity = quantity });
                }

                return exVal;
            });

            return Task.CompletedTask;
        }

        public Task EmptyCartAsync(string userId)
        {
            Console.WriteLine($"EmptyCartAsync called with userId={userId}");
            userCartItems[userId] = new Msdemo.Cart();

            return Task.CompletedTask;
        }

        public Task<Msdemo.Cart> GetCartAsync(string userId)
        {
            Console.WriteLine($"GetCartAsync called with userId={userId}");
            Msdemo.Cart cart = null;
            if (!userCartItems.TryGetValue(userId, out cart))
            {
                Console.WriteLine($"No carts for user {userId}");
                return Task.FromResult(emptyCart);
            }

            return Task.FromResult(cart);
        }

        public bool Ping()
        {
            return true;
        }
    }
}