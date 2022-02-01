-- The following report shows the miner and its active software/algorithm.

SELECT m.name AS "Miner", ms.name AS "Active Software", a.name AS "Algo"
FROM miners m 
LEFT JOIN miner_software_algos msa ON
	msa.id = m.miner_software_algo_id 
LEFT JOIN miner_softwares ms ON 
	ms.id = msa.miner_software_id 
LEFT JOIN algorithms a ON 
	a.id = msa.algorithm_id 
WHERE miner_software_algo_id IS NOT NULL;