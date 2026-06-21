package sherlocks

type SherlockTask struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MaskedFlag  string `json:"masked_flag"`
	Hint        string `json:"hint"`
	Completed   bool   `json:"completed"`
}

type SherlockDataTasks struct {
	Tasks []SherlockTask `json:"data"`
}

type SherlockElement struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Difficulty   string `json:"difficulty"`
	CategoryName string `json:"category_name"`
	IsOwned      bool   `json:"is_owned"`
}

type SherlockData struct {
	Data []SherlockElement `json:"data"`
}

type SherlockNameID struct {
	Name       string
	ID         int
	Difficulty string
	Category   string
	Owned      bool
}

type DownloadFile struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}
