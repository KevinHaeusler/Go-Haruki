package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KevinHaeusler/go-haruki/bot"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN missing in environment")
	}

	guildID := os.Getenv("GUILD_ID") // optional

	if err := bot.Start(token, guildID); err != nil {
		log.Fatal(err)
	}
	defer bot.Stop()

	// keep process alive until Ctrl+C / SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
