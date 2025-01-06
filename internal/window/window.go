package window

import (
	"context"
	"fmt"

	"github.com/joshuarubin/go-sway"
)

func FindWindow(title string) (int64, error) {
	client, err := sway.New(context.Background())
	if err != nil {
		return 0, err
	}

	tree, err := client.GetTree(context.Background())
	if err != nil {
		return 0, err
	}

	var findNode func(node *sway.Node) *sway.Node
	findNode = func(node *sway.Node) *sway.Node {
		if node.AppID != nil && *&node.Name == title {
			return node
		}
		for _, n := range node.Nodes {
			if found := findNode(n); found != nil {
				return found
			}
		}
		for _, n := range node.FloatingNodes {
			if found := findNode(n); found != nil {
				return found
			}
		}
		return nil
	}

	if node := findNode(tree); node != nil {
		return node.ID, nil
	}
	return 0, nil
}

func FocusWindow(windowID int64) error {
	client, err := sway.New(context.Background())
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`[con_id="%d"] focus`, windowID)
	_, err = client.RunCommand(context.Background(), cmd)
	return err
}
