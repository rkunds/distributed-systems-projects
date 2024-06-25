# Distributed Transactions

Install necessary dependencies, mainly `Go` `v1.26.6` and `gRPC`. 

This is a distributed transaction system, supporting full ACI (not durable :/) properties. Accounts are distributed across 5 nodes, or branches, and accounts on a specific branch exist only on that node. Clients randomly pick one of the five servers to communicate with as the frontend, and the server acts as the coordinator for that transaction. Failures are not handled. 

Transactions start with the `BEGIN` command, and users can `DEPOSIT` money to `{A, B, C, D, E}.{account_name}`, or `WITHDRAW`. One can view balance using the `BALANCE` command, with the branch and account id following.

On illegal commands, such as finding the balance of an account that does not exist, the transaction aborts. Account balances can be negative, but a transaction cannot be commited if an account balance is negative. 

To abort the current transaction, enter `ABORT`. 

Timestamped concurrency control is used as the deadlock prevention strategy.