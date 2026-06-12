package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerEntraMcpTools adds all Entra provisioning MCP tools to the server.
func registerEntraMcpTools(s *server.MCPServer) {
	s.AddTool(provisionUserTool())
	s.AddTool(deprovisionUserTool())
	s.AddTool(updateUserStatusTool())
	s.AddTool(listUsersTool())
}

func provisionUserTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("provision_user",
		mcp.WithDescription("Create a new user account in Microsoft Entra (Azure AD). Returns the new user's object ID and UPN."),
		mcp.WithString("username",
			mcp.Required(),
			mcp.Description("Unique username / mail nickname (e.g. alice.smith). Used as the mailNickname and to derive the UPN if email is not a UPN."),
		),
		mcp.WithString("email",
			mcp.Required(),
			mcp.Description("Email address / UPN for the new account (e.g. alice@yourtenant.onmicrosoft.com)."),
		),
		mcp.WithString("first_name",
			mcp.Description("User's given name."),
		),
		mcp.WithString("last_name",
			mcp.Description("User's family name / surname."),
		),
		mcp.WithString("password",
			mcp.Required(),
			mcp.Description("Initial password. Must meet Entra password complexity requirements."),
		),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username, err := req.RequireString("username")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		email, err := req.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		password, err := req.RequireString("password")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		firstName := req.GetString("first_name", "")
		lastName := req.GetString("last_name", "")

		log.Printf("tool=provision_user username=%s email=%s", username, email)
		result, err := CreateEntraUser(username, email, firstName, lastName, password)
		if err != nil {
			log.Printf("tool=provision_user error: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Entra error: %v", err)), nil
		}
		log.Printf("tool=provision_user success: %s", result)
		return mcp.NewToolResultText(result), nil
	}
	return tool, handler
}

func deprovisionUserTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("deprovision_user",
		mcp.WithDescription("Permanently delete a user account from Microsoft Entra by their email / UPN."),
		mcp.WithString("email",
			mcp.Required(),
			mcp.Description("Email address or UPN of the user to delete."),
		),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := req.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		log.Printf("tool=deprovision_user email=%s", email)
		result, err := DeleteEntraUser(email)
		if err != nil {
			log.Printf("tool=deprovision_user error: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Entra error: %v", err)), nil
		}
		log.Printf("tool=deprovision_user success: %s", result)
		return mcp.NewToolResultText(result), nil
	}
	return tool, handler
}

func updateUserStatusTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("update_user_status",
		mcp.WithDescription("Enable or disable a user account in Microsoft Entra (sets accountEnabled)."),
		mcp.WithString("email",
			mcp.Required(),
			mcp.Description("Email address or UPN of the user to update."),
		),
		mcp.WithBoolean("enabled",
			mcp.Required(),
			mcp.Description("Set to true to enable the account, false to disable it."),
		),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := req.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		enabled := req.GetBool("enabled", true)
		log.Printf("tool=update_user_status email=%s enabled=%v", email, enabled)
		result, err := UpdateEntraUserStatus(email, enabled)
		if err != nil {
			log.Printf("tool=update_user_status error: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Entra error: %v", err)), nil
		}
		log.Printf("tool=update_user_status success: %s", result)
		return mcp.NewToolResultText(result), nil
	}
	return tool, handler
}

func listUsersTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("list_users",
		mcp.WithDescription("List or search user accounts in Microsoft Entra. Returns id, UPN, displayName, mail, and accountEnabled for each matching user."),
		mcp.WithString("filter",
			mcp.Description("Optional OData $filter expression (e.g. 'startsWith(mail, \"alice\")'). Leave empty to list all users."),
		),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filter := req.GetString("filter", "")
		log.Printf("tool=list_users filter=%q", filter)
		result, err := ListEntraUsers(filter)
		if err != nil {
			log.Printf("tool=list_users error: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Entra error: %v", err)), nil
		}
		return mcp.NewToolResultText(result), nil
	}
	return tool, handler
}
