package provider

type SummaryFieldsAPIModel struct {
	Organization SummaryAPIModel `json:"organization,omitempty"`
	Inventory    SummaryAPIModel `json:"inventory,omitempty"`
}

// If we end up pulling in more summary_fields that have other information, we can split
// them out to their own structs at that time.
type SummaryAPIModel struct {
	Id          int64  `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RelatedAPIModel struct {
	NamedUrl string `json:"named_url,omitempty"`
}
