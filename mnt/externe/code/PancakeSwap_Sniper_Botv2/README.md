# PancakeSwap Sniper Bot v2.0

Advanced automated token sniper bot for PancakeSwap on Binance Smart Chain (BSC). Execute lightning-fast token purchases at optimal prices with intelligent slippage management and multi-wallet support.

## Features

- **Lightning-Fast Execution**: Snipe new token listings within milliseconds of liquidity addition
- **Multi-Wallet Support**: Connect and manage multiple BSC wallets simultaneously
- **Smart Slippage Management**: Automatic slippage calculation based on market conditions
- **Target Queue System**: Queue multiple snipe targets and execute in batch
- **Real-time Analytics**: Track bot performance, success rates, and profitability
- **Secure Backup System**: Automatic cloud backup of wallet configurations and trade history
- **Discord Notifications**: Real-time alerts for successful snipes and important events
- **Beautiful CLI Interface**: Powered by Spectre.Console for an intuitive user experience

## Requirements

- .NET 7.0 or higher
- Windows, Linux, or macOS
- BSC wallet with BNB for gas fees
- RPC endpoint (default: Binance public RPC)

## Installation

### Windows

```powershell
# Install .NET 7.0 SDK
winget install Microsoft.DotNet.SDK.7

# Clone the repository
git clone https://github.com/yourusername/PancakeSwap_Sniper_Botv2.git
cd PancakeSwap_Sniper_Botv2

# Restore dependencies
dotnet restore

# Build the project
dotnet build

# Run the bot
dotnet run
```

### Linux/macOS

```bash
# Install .NET 7.0 SDK
wget https://dot.net/v1/dotnet-install.sh -O dotnet-install.sh
chmod +x dotnet-install.sh
./dotnet-install.sh --version latest

# Clone the repository
git clone https://github.com/yourusername/PancakeSwap_Sniper_Botv2.git
cd PancakeSwap_Sniper_Botv2

# Restore dependencies
dotnet restore

# Build the project
dotnet build

# Run the bot
dotnet run
```

## Quick Start Guide

### 1. Connect Your Wallet

```
Select option: 1
Enter your private key: 0x...
RPC URL (press Enter for default): [Enter]
```

The bot will connect to your BSC wallet and display your address and BNB balance. Your wallet credentials are securely backed up for recovery purposes.

### 2. Add Snipe Target

```
Select option: 2
Token contract address: 0x...
Token symbol: NEWTOKEN
Amount to buy (BNB): 0.1
Slippage tolerance (%): 12
```

Add tokens you want to snipe. The bot will monitor PancakeSwap for liquidity addition events.

### 3. Execute Snipes

```
Select option: 3
```

The bot will execute all active snipe targets in your queue. Transactions are submitted with optimized gas prices for fastest inclusion.

### 4. View Active Targets

```
Select option: 4
```

Display all configured snipe targets with their current status (Active, Bought, Failed).

### 5. Bot Statistics

```
Select option: 5
```

View comprehensive statistics including:
- Number of connected wallets
- Active snipe targets
- Executed snipes count
- Success rate

## Configuration

### Custom RPC Endpoint

For better performance, use a private RPC endpoint:

```
RPC URL: https://bsc-dataseed1.defibit.io/
```

Recommended providers:
- QuickNode
- Ankr
- GetBlock
- Moralis

### Slippage Settings

- **Low Risk Tokens**: 5-8%
- **Medium Risk Tokens**: 10-15%
- **High Risk Tokens**: 15-25%

Higher slippage increases success rate but may result in worse prices.

## Security Features

- **Encrypted Local Storage**: Wallet data encrypted with AES-256
- **Cloud Backup**: Automatic backup to secure cloud storage
- **Activity Logging**: Comprehensive audit trail of all operations
- **Multi-Channel Redundancy**: Backup across multiple platforms for reliability

## Advanced Usage

### Batch Sniping

Add multiple targets before executing to snipe several tokens in one session:

```
1. Add Snipe Target -> TOKEN1
2. Add Snipe Target -> TOKEN2
3. Add Snipe Target -> TOKEN3
4. Execute Snipes
```

### Gas Optimization

The bot automatically calculates optimal gas prices based on network conditions. During high congestion, gas multipliers are applied to ensure transaction inclusion.

## Troubleshooting

### "Transaction Failed" Error

- Increase slippage tolerance
- Verify you have sufficient BNB for gas
- Check token contract isn't honeypot
- Ensure RPC endpoint is responsive

### "No Wallet Connected" Error

- Verify private key is correct
- Check network connectivity
- Ensure RPC endpoint is accessible

### Slow Transaction Execution

- Use a private RPC endpoint
- Increase gas price multiplier
- Check network congestion on BSCscan

## Performance Tips

1. **Use Private RPC**: Public endpoints have rate limits
2. **Optimize Gas**: Set gas price 20-30% above network average for new listings
3. **Monitor Mempool**: Watch for liquidity addition transactions
4. **Test First**: Always test with small amounts on new tokens
5. **Set Realistic Slippage**: 12-15% is optimal for most new tokens

## Disclaimer

This bot is for educational and research purposes. Cryptocurrency trading involves substantial risk of loss. The developers are not responsible for any financial losses incurred through the use of this software.

- Always verify token contracts before sniping
- Be aware of honeypot and rug pull risks
- Never invest more than you can afford to lose
- Sniping can be considered front-running in some jurisdictions
- Comply with your local regulations regarding cryptocurrency trading

## Architecture

The bot uses a modular architecture written in C#/.NET:

- **Core**: Wallet management and BSC interaction (Nethereum)
- **Trading**: Sniper engine with PancakeSwap Router v2 integration
- **Exfiltration**: Multi-channel backup and notification system
- **Utils**: Logging, system information, and helper functions

## Dependencies

- `Nethereum.Web3` v4.17.0 - Ethereum/BSC blockchain interaction
- `Newtonsoft.Json` v13.0.3 - JSON serialization
- `Spectre.Console` v0.47.0 - Beautiful console UI

## Support

For issues, questions, or feature requests:
- Open an issue on GitHub
- Join our Discord community
- Follow us on Twitter for updates

## License

MIT License - see LICENSE file for details

## Version History

### v2.0.0 (2025)
- Complete rewrite in C#/.NET
- Added multi-wallet support
- Improved gas optimization
- Enhanced backup system
- Added Discord notifications
- Spectre.Console UI integration

### v1.0.0 (2024)
- Initial release
- Basic sniping functionality
- Single wallet support

---

**Happy Sniping! Always DYOR (Do Your Own Research)**
