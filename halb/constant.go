package halb

type LoadBalancingStrategy string

const (
	RoundRobin LoadBalancingStrategy = "round_robin"
	LeastConn  LoadBalancingStrategy = "least_conn"
)
