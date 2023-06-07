package telebot

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var DefaultTimeoutForHandlingAlbum = time.Millisecond * 500

type AlbumHandlerFunc func(cs []Context) error

func (f AlbumHandlerFunc) ToHandlerFunc() HandlerFunc {
	return func(c Context) error {
		return f([]Context{c})
	}
}

type albumHandler struct {
	Group   *Group
	Handler AlbumHandlerFunc
	Timeout time.Duration

	albums        map[string][]Context
	registerMutex sync.Mutex
}

func (b *Bot) HandleAlbum(handler AlbumHandlerFunc, m ...MiddlewareFunc) {
	b.Group().HandleAlbum(handler, m...)
}

func (g *Group) HandleAlbum(handler AlbumHandlerFunc, m ...MiddlewareFunc) {
	albumHandler := albumHandler{
		Group:         g,
		Handler:       handler,
		Timeout:       DefaultTimeoutForHandlingAlbum,
		albums:        map[string][]Context{},
		registerMutex: sync.Mutex{},
	}
	albumHandler.Group.Handle(OnMedia, func(ctx Context) error { return albumHandler.Register(ctx) }, m...)
}

func (handler *albumHandler) Register(ctx Context) error {
	defer handler.registerMutex.Unlock()
	handler.registerMutex.Lock()

	id := mediaGroupToId(ctx.Message())
	if _, contains := handler.albums[id]; !contains {
		handler.albums[id] = []Context{ctx}

		go handler.delayHandling(ctx, id)
	} else {
		handler.albums[id] = append(handler.albums[id], ctx)
	}

	return nil
}

func (handler *albumHandler) delayHandling(ctx Context, id string) {
	message := ctx.Message()
	defer func() {
		delete(handler.albums, id)
		if r := recover(); r != nil {
			ctx.Bot().OnError(errors.New(fmt.Sprintf("%v", r)), ctx.Bot().NewContext(Update{Message: deepCopyViaJsonSorryJesusChrist(message)}))
		}
	}()
	if message.AlbumID != "" { // no need to delay handling of single medias
		time.Sleep(handler.Timeout)
	}
	contexts := handler.albums[mediaGroupToId(message)]
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Message().ID < contexts[j].Message().ID
	})
	err := handler.Handler(contexts)
	if err != nil {
		ctx.Bot().OnError(err, ctx.Bot().NewContext(Update{Message: deepCopyViaJsonSorryJesusChrist(message)}))
	}
}

func deepCopyViaJsonSorryJesusChrist[T any](obj *T) *T {
	buff, err := json.Marshal(obj)
	if err != nil {
		panic(err) // unexpected behavior
	}
	var copied T
	err = json.Unmarshal(buff, &copied)
	if err != nil {
		panic(err) // much more unexpected behavior
	}
	return &copied
	return obj
}

func mediaGroupToId(msg *Message) string {
	if msg.AlbumID != "" {
		return msg.AlbumID
	} else {
		return fmt.Sprintf("%d_%d", msg.Chat.ID, msg.ID)
	}
}
