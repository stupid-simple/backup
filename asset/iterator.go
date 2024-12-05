package asset

import "context"

func NewChannelIterator(rootPath string, ch <-chan Asset) AssetIterator {
	return channelIterator{rootPath, ch}
}

type channelIterator struct {
	rootPath string
	ch       <-chan Asset
}

func (c channelIterator) RootPath() string {
	return c.rootPath
}

func (c channelIterator) IterateAssets(ctx context.Context) <-chan Asset {
	return c.ch
}
