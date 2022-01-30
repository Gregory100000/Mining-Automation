// Database Connectivity
host="localhost"
port="5432"
database="mining"
user="postgres"
password="whateves"
timezone="America/Chicago"

// Miner Stats Configuration
minerName="YourMiner"  // An identifier for the mining hardware.
// The password field to use for the pool. Often, this is used to handle certain settings etc.
// Down the road, this may need to be configurable.
poolPassword = "Probably Need This"
// Optional, as some software requires to connect to a pool which may require a wallet
wallet=""  
// If this is 1, estimates will be used for optimization instead of 24 hour actual profit.
useEstimates=1
// If this is 1, the computer will be rebooted if the mining software dies unexpectedly.
rebootOnFailure=0
// Time in seconds to wait before checking for the next possible optimization.
optimizationCheckTime = 3600 // 3600 seconds = 1 hour

// E-mail Server Settings (SMTP)
emailServer="" // If this is set, e-mails will be attempted.
emailPort=""
emailUser=""
emailPassword=""
emailFrom="" // The from address to use
emailTo="" // The destination address
