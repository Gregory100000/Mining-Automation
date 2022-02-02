package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/shirou/gopsutil/process"
	"gorm.io/gorm"

	. "github.com/GregoryUnderscore/Mining-Automation-Shared/database"
	. "github.com/GregoryUnderscore/Mining-Automation-Shared/models"
	. "github.com/GregoryUnderscore/Mining-Automation-Shared/utils/pools"
)

// ====================================
// Configuration File (Automate.hcl)
// ====================================
type Config struct {
	// Database Connectivity
	Host     string `hcl:"host"`     // The server hosting the database
	Port     string `hcl:"port"`     // The port of the database server
	Database string `hcl:"database"` // The database name
	User     string `hcl:"user"`     // The user to use for login to the database server
	Password string `hcl:"password"` // The user's password for login
	TimeZone string `hcl:"timezone"` // The time zone where the program is run

	// Miner Specific Settings
	MinerName    string `hcl:"minerName"`    // The name of the mining hardware
	PoolPassword string `hcl:"poolPassword"` // The password field for the pool
	Wallet       string `hcl:"wallet"`       // The wallet to use for mining
	// If this is 1, estimates will be used for optimization instead of 24 hour actual profit.
	UseEstimates uint8 `hcl:"useEstimates"`
	// If this is 1, the computer will be rebooted if the mining software dies unexpectedly.
	RebootOnFailure uint8 `hcl:"rebootOnFailure"`
	// Time in seconds to wait before checking for the next possible optimization.
	OptimizationCheckTime uint32 `hcl:"optimizationCheckTime"`

	// E-mail Server Settings (SMTP)
	EmailServer   string `hcl:"emailServer"`
	EmailPort     string `hcl:"emailPort"`
	EmailUser     string `hcl:"emailUser"` // The user for login
	EmailPassword string `hcl:"emailPassword"`
	EmailFrom     string `hcl:"emailFrom"` // The from address
	EmailTo       string `hcl:"emailTo"`   // The recipient
}

func main() {
	const configFileName = "Automate.hcl" // The name of the config file
	var config Config                     // The configuration data will be here
	var thisMiner Miner                   // The miner that is being optimized

	// Grab the configuration details for the database connection. These are stored in ZergPoolData.hcl.
	err := hclsimple.DecodeFile(configFileName, nil, &config)
	if err != nil {
		log.Fatalf("Failed to load config file "+configFileName+".\n", err)
	}

	// Connect to the database and create/validate the schema.
	db := Connect(config.Host, config.Port, config.Database, config.User, config.Password,
		config.TimeZone)
	VerifyAndUpdateSchema(db)

	// Open the new database transaction.
	tx := db.Begin()

	defer func() { // Ensure transaction rollback on panic
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	log.Println("Creating records required for operations...")
	minerID := VerifyMiner(tx, config.MinerName)
	// Grab the miner record.
	tx.Where("id = ?", minerID).Find(&thisMiner)
	if (Miner{}) == thisMiner {
		log.Fatalf("Unable to locate this miner in the database: " + config.MinerName)
	}
	err = tx.Commit().Error // Commit changes to the database
	if err != nil {
		log.Fatalf("Issue committing changes.\n", err)
	}

	// Determine the best software/algorithm for this miner.
	log.Println("Determining optimal software/algo combination...")
	bestSoftwareAlgo := getBestSoftwareAlgo(db, minerID, config.UseEstimates)
	// Generate parameters and get the file path for the first run.
	params, filePath := changeAlgoGetParams(db, &thisMiner, bestSoftwareAlgo, config)
	// Kick off the mining software for the first time.
	proc := openProcess(filePath, params)

	totalSecondsSlept := uint64(0) // Tracks the total time slept to know when to check for optimization
	processCheckTime := 30         // Wait 30 seconds in between activity checks
	// Endlessly loop and check for better optimizations after the configured time.
	for {
		// Time to check for an optimization.
		if totalSecondsSlept > 0 && (totalSecondsSlept%uint64(config.OptimizationCheckTime) == 0) {
			optimizationAlgo := getBestSoftwareAlgo(db, minerID, config.UseEstimates)
			// Is the best algo a change?
			if optimizationAlgo.ID != thisMiner.MinerSoftwareAlgoID {
				proc.Kill() // Stop the current mining process.
				proc.Wait() // Wait for everything to stop. Also releases resources.
				// Generate parameters and get the file path for the next run.
				// Also, set the active software/algo on the miner.
				params, filePath = changeAlgoGetParams(db, &thisMiner, optimizationAlgo,
					config)
				// Kick off the mining software again.
				proc = openProcess(filePath, params)
			}
		} else {
			// Wait 30 seconds and then validate the process still exists.
			time.Sleep(time.Duration(processCheckTime) * time.Second)
			totalSecondsSlept += uint64(processCheckTime)
			exists, _ := process.PidExists(int32(proc.Pid))
			if exists {
				continue
			}

			// Process exited probably on error.
			// Ensure everything has been cleared.
			proc.Kill() // Stop any current mining process.
			proc.Wait() // Wait for everything to stop. Also releases resources.

			// Kick off the mining software again.
			proc = openProcess(filePath, params)
		}
	}
}

// Change the algorithm on the miner in the database and also generate the parameters necessary for
// opening the mining software with the optimized algorithm.
// @param db - The active database connect
// @param miner - A pointer to the active miner. The active algorithm changes, thus pass by reference.
// @param bestSoftwareAlgo - The optimized algo that should now be used
// @param config - The configuration details from the HCL config file
// @returns - A tuple of parameters for running with the mining software and the file path to the mining
//    software.
func changeAlgoGetParams(db *gorm.DB, miner *Miner, bestSoftwareAlgo MinerSoftwareAlgos,
	config Config) ([]string, string) {
	var minerSoft MinerSoftware
	var algo Algorithm
	var minerSoftDetails MinerMinerSoftware

	// Open the new database transaction.
	tx := db.Begin()
	defer func() { // Ensure transaction rollback on panic
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Locate the necessary records to proceed.
	tx.Where("id = ?", bestSoftwareAlgo.MinerSoftwareID).Find(&minerSoft)
	tx.Where("id = ?", bestSoftwareAlgo.AlgorithmID).Find(&algo)
	if (MinerSoftware{}) == minerSoft || (Algorithm{}) == algo {
		log.Fatalf("Miner software algo has a bad software or algo link: " +
			fmt.Sprint(bestSoftwareAlgo.ID))
	}
	tx.Where("miner_id = ? AND miner_software_id = ?", miner.ID, minerSoft.ID).
		Find(&minerSoftDetails)
	if (MinerMinerSoftware{}) == minerSoftDetails {
		log.Fatalf("No file path found for miner software: " + minerSoft.Name)
	}
	log.Println("Found new optimal software/algorithm...")
	body := "Software: " + minerSoft.Name + "\r\n" +
		"Algo: " + algo.Name + "\r\n"
	log.Print(body)
	// Send an e-mail notification if the server is set.
	if len(config.EmailServer) > 0 {
		sendEmail(config.MinerName+": New Optimal", body, config)
	}

	miner.MinerSoftwareAlgoID = bestSoftwareAlgo.ID
	tx.Save(miner) // Save the algo change.

	// Generate pool URL.
	poolURL := GeneratePoolURL(tx, bestSoftwareAlgo.AlgorithmID)

	// Create the core parameter structure for the miner software.
	// This includes the algorithm parameter requirements and any other
	// requirements for actual operations.
	// Create the full parameter list
	params := []string{minerSoft.Name,
		minerSoft.AlgoParam, algo.Name,
		minerSoft.PoolParam, poolURL,
		minerSoft.WalletParam, config.Wallet,
		minerSoft.PasswordParam, config.PoolPassword,
	}
	// Process any additional parameters in the catch-all other parameters.
	if len(minerSoft.OtherParams) > 0 {
		otherParams := strings.Split(minerSoft.OtherParams, " ")
		params = append(params, otherParams...)
	}
	// Some algorithms have parameters specific to them.
	if len(bestSoftwareAlgo.ExtraParams) > 0 {
		extraParams := strings.Split(bestSoftwareAlgo.ExtraParams, " ")
		params = append(params, extraParams...)
	}

	err := tx.Commit().Error // Commit changes to the database
	if err != nil {
		log.Fatalf("Issue committing changes.\n", err)
	}

	return params, minerSoftDetails.FilePath
}

// Determine the best software/algo for a miner by examining the most profitable combination.
// @param tx - The active database connection
// @param minerID - The ID for the active miner
// @param useEstimates - If this is 1, the 24 hour estimate is utilized for profit comparisons. If 0, the
//    24-hour actuals are used.
// @returns The best software/algo
func getBestSoftwareAlgo(db *gorm.DB, minerID uint64, useEstimates uint8) MinerSoftwareAlgos {
	// Define subquery to get the average work_per_second for the miner/software/algos.
	subAvgWork :=
		db.Select("miner_id, miner_software_id, algorithm_id, "+
			"AVG(work_per_second) AS average_work, mh_factor").
			Where("miner_id = ?", minerID).
			Group("miner_id, miner_software_id, algorithm_id, mh_factor").
			Table("miner_stats")
	// Get subquery for the latest mining stats for this miners/software/algos.
	subLatestStat :=
		db.Select("miner_id, miner_software_id, algorithm_id, MAX(id) AS latest_stat_id").
			Where("miner_id = ?", minerID).
			Group("miner_id, miner_software_id, algorithm_id").
			Table("miner_stats")
	// Get subqyery for the latest pool stats for each algo pool.
	subLatestPoolStat :=
		db.Select("MAX(id) AS id").
			Group("pool_id").
			Table("pool_stats")

	// Use estimates to determine profit optimization.
	orderLogic := "price*profit_estimate*(average_stat.mh_factor / pools.mh_factor)*average_work DESC"
	// Use 24-hour actuals if the config directs.
	if useEstimates == 0 {
		orderLogic = "price*0.001*profit_actual24_hours*(average_stat.mh_factor / pools.mh_factor)*" +
			"average_work DESC"
	}
	// Get all the mining stats for this miner and ensure they are also linked to a pool.
	var bestMinerSoftwareAlgo MinerSoftwareAlgos
	db.Table("miners").
		Select("miner_software_algos.*").
		Joins("INNER JOIN (?) latest_stat ON latest_stat.miner_id = miners.id", subLatestStat).
		Joins("INNER JOIN miner_stats ON miner_stats.id = latest_stat.latest_stat_id").
		Joins("INNER JOIN miner_softwares ON latest_stat.miner_software_id = miner_softwares.id").
		Joins("INNER JOIN algorithms ON algorithms.id = latest_stat.algorithm_id").
		Joins("INNER JOIN miner_software_algos ON miner_software_algos.algorithm_id = algorithms.id "+
			"AND miner_software_algos.miner_software_id = miner_softwares.id").
		// Inner join these to the pool algos.
		Joins("INNER JOIN pools ON pools.algorithm_id = algorithms.id").
		Joins("INNER JOIN pool_stats ON pool_stats.pool_id = pools.id").
		Joins("INNER JOIN (?) latest_pool_stat ON latest_pool_stat.id = pool_stats.id",
			subLatestPoolStat).
		// Get the latest Bitcoin price.
		Joins("INNER JOIN coin_prices ON pool_stats.coin_price_id = coin_prices.id").
		Joins("INNER JOIN (?) average_stat ON average_stat.miner_id = miners.id "+
			"AND average_stat.miner_software_id = miner_softwares.id "+
			"AND average_stat.algorithm_id = algorithms.id", subAvgWork).
		Where("miners.id = ? AND (do_not_use IS NULL OR do_not_use = FALSE)", minerID).
		Order(orderLogic).
		Limit(1).
		Find(&bestMinerSoftwareAlgo)
	// Error out if nothing was found. Probably there is not enough statistics in the database.
	if (MinerSoftwareAlgos{}) == bestMinerSoftwareAlgo {
		log.Fatalf("Could not determine an optimization for this miner. Try running the pool stats " +
			"program to load pool statistics (e.g. zerg.exe), or try running the miner " +
			"statistics program to load miner statistics (i.e. minerStats.exe).")
	}
	return bestMinerSoftwareAlgo
}

// Send an e-mail using the configuration to obtain login details, recipient, etc.
// @param subject - The subject for the e-mail
// @param body - The body of the e-mail
// @param config - The configuration object with all the e-mail login details etc.
func sendEmail(subject string, body string, config Config) {
	// Create the authentication object
	auth := smtp.PlainAuth("", config.EmailUser, config.EmailPassword, config.EmailServer)

	// Prepare the e-mail for transmission.
	to := []string{config.EmailTo}
	msg := []byte("To: " + config.EmailTo + "\r\n" +
		"From: " + config.EmailFrom + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" + body + "\r\n")

	// Transmit.
	err := smtp.SendMail(config.EmailServer+":"+config.EmailPort, auth, config.EmailFrom, to, msg)
	if err != nil { // Do not fatally error in case the e-mail server is just temporarily down.
		log.Printf("Problem sending e-mail notification: \n", err)
	}
}

// Open a process and get back the pointer to it.
// @param filePath - The path to the executable to open
// @param params - The parameters to use for the process
func openProcess(filePath string, params []string) *os.Process {
	output := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	// Open the miner program in a child process.
	attr := &os.ProcAttr{
		"",
		nil,
		output,
		&syscall.SysProcAttr{},
	}
	proc, error := os.StartProcess(filePath, params, attr)
	if error != nil {
		log.Fatalf("Unable to start mining software.\n", error)
	}
	return proc
}
