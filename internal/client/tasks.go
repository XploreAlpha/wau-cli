package client

import (
	"context"
	"fmt"
)

// SubmitTask submits a new task to the kernel.
func (c *Client) SubmitTask(ctx context.Context, req *TaskSubmitRequest) (*TaskSubmitResponse, error) {
	var resp TaskSubmitResponse
	if err := c.Post(ctx, "/registry/tasks/submit", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTask returns task details by ID.
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	var resp Task
	if err := c.Get(ctx, fmt.Sprintf("/registry/tasks/%s", taskID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
