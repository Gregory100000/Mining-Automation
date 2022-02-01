# ** Optimization Automation of CPU/GPU Miners **

### **Summary**
Enhance CPU/GPU miner profitability in coordination with the following utilities:
Zergpool Statistics Storage - https://github.com/GregoryUnderscore/Mining-Automation-ZergPool.com
Miner Statistics Assessor - https://github.com/GregoryUnderscore/Mining-Automation-Miner-Stats

After loading Zergpool statistics and assessing mining software (e.g. cpuminer-opt) on a miner, the most profitable algorithm 
can automatically be determined/utilized.

### **Important**
Pool provider statistics are required for profitability estimates/actuals. Before using this, please see the instructions at: https://github.com/GregoryUnderscore/Mining-Automation-ZergPool.com
Also, miner statistics must be assessed. See the instructions at: https://github.com/GregoryUnderscore/Mining-Automation-Miner-Stats

Sometimes a coin forks and has issues. In those situations, it can incorrectly show as the most profitable. If this
happens, you can set the do_not_use field to true, and the automation program will skip it. This field is in the miner_software_algos table. Down the road, a utility may be created to make this easier for users that do not feel comfortable
manually editing database tables.

### **Description**
ZergPool provides several useful statistics for every pool they host. This allows a miner to calculate projections
and possible profit opportunities. However, to properly calculate these projections, a miner's hash rate must be calculated
for all supported algorithms. This can be a painstaking process when done manually. This program in coordination with 2 others (listed formerly) automate most of this.

This has been tested on Linux and Windows.

### **How to Use**

1. Follow the instructions first at https://github.com/GregoryUnderscore/Mining-Automation-ZergPool.com
2. Follow the instructions next at https://github.com/GregoryUnderscore/Mining-Automation-Miner-Stats
3. Download the latest release and extract it to a folder.
4. Update the Automate.hcl file with the appropriate details.
4. Run the automate.exe or automate (on Linux). It will automatically determine the most profitable algorithm based on Zergpool's 
estimates and the miner statistics.

### **Included Reports**
In the sql folder are SQL reports. There is a report to see the active algorithm running on each miner.

### No affiliation with Zergpool.com
