# Decentralized Total Ordering

We use `gRPC` and `Go` `v1.21.6`. 

Specify a cluster of nodes in a text file with the number of nodes, followed by the node name and host/port. See `config/local{3,8}.txt` for an example. Start nodes by compiling the binary and running it with the node name as well as the path to the config file: `./node {node_name} {path_to_config}`. 

The system keeps tracks of accounts of a lowerchase alphabetical string with a non-negative balance. Transactions consist of `DEPOSIT`s and `TRANSFER`s for individual accounts and between accounts. Any node can initiate a transaction, and all other nodes will process the transactions in the same order. This is achieved using the ISIS total ordering algorithm and ordered multicast between nodes. Nodes can fail, but the system will be tolerant as long as a majority of nodes are active. 