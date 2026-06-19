package main

import "fmt"

func main() {
    feed, err := fetchFeed("https://kobzol.github.io/feed.xml")
    if err != nil {
        fmt.Println("ERROR:", err)
        return
    }
    fmt.Println("Title:", feed.Title)
    fmt.Printf("Articles: %d\n\n", len(feed.Articles))
    for i, a := range feed.Articles {
        if i >= 3 { break }
        fmt.Printf("--- %s (%s)\n", a.Title, a.Published)
        preview := a.Content
        if len(preview) > 200 { preview = preview[:200] + "..." }
        fmt.Println(preview)
        fmt.Println()
    }
}
