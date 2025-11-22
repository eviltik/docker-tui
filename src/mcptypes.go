package main

// GetLogsArgs defines arguments for the get_logs tool
type GetLogsArgs struct {
	Containers []string `json:"containers,omitempty" description:"Container names or IDs (supports partial matches). Leave empty to search across ALL containers."`
	Filter     string   `json:"filter,omitempty" description:"Keyword or regex pattern to filter log lines"`
	IsRegex    bool     `json:"is_regex,omitempty" description:"Treat filter as regex (default: false, substring search)"`
	Lines      int      `json:"lines,omitempty" description:"Maximum lines per container (default: 100, max: 10000)"`
	Tail       bool     `json:"tail,omitempty" description:"Return most recent lines (default: true)"`
}

// ListContainersArgs defines arguments for the list_containers tool
type ListContainersArgs struct {
	All         bool   `json:"all,omitempty" description:"Include stopped containers (default: false, only running)"`
	NameFilter  string `json:"name_filter,omitempty" description:"Filter by container name (case-insensitive substring)"`
	StateFilter string `json:"state_filter,omitempty" description:"Filter by state (running, exited, paused, restarting, etc.)"`
}

// GetStatsArgs defines arguments for the get_stats tool
type GetStatsArgs struct {
	Containers []string `json:"containers" description:"Container names or IDs (supports partial matches)"`
	History    bool     `json:"history,omitempty" description:"Include 10-value CPU history (default: false)"`
}

// ContainerActionArgs defines arguments for container action tools (start, stop, restart)
type ContainerActionArgs struct {
	Containers []string `json:"containers" description:"Container names or IDs to act on (supports partial matches)"`
}
