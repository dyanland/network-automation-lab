#Rollback Procedure
## Trigger Point
- BGP convergence > 2 minutes
- OSPF adjacency failures
- Packet loss > 0.1%

## Steps
1. Revert BGP weight changes
2. Verify Traffic flow to legacy core
3. Remove new BGP peerings
4. Restrore configurations from backup

## Validation
- Check OSPF Neighbor: show ip ospf neighbor
- Verify BGP Status: show bgp summary
- Test connectivity: ping critical endpoints
