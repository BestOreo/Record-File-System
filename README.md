# Record-File-System

### network topology
![img](https://github.com/BestOreo/Record-File-System/blob/master/topo/topo1.png)


### start in three different terminals
M1
```
go run miner.go config.json
```
M2
```
go run miner.go config2.json
```
M3
```
go run miner.go config3.json
```

### client operation
1. create a file
```
go run touch.go [filename]
```
2. append a record
```
go run append.go [filename] <string>
```
3. list files
```
go run ls.go -a
```
4. head
```
go run head.go <k> <fname>
```
5. tail
```
go run tail.go <k> <fname>
```

