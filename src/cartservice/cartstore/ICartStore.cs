using System.Threading.Tasks;

namespace cartservice.cartstore
{
    public interface ICartStore
    {
        Task InitializeAsync();
        
        Task AddItemAsync(string userId, string productId, int quantity);
        Task EmptyCartAsync(string userId);

        Task<Msdemo.Cart> GetCartAsync(string userId);

        bool Ping();
    }
}