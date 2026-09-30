package grpc

import (
	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"slices"
)

func accountPlansToProto(plans []sdk.AccountPlan) []*pb.AccountPlanProto {
	if plans == nil {
		return nil
	}
	result := make([]*pb.AccountPlanProto, 0, len(plans))
	for _, plan := range plans {
		result = append(result, &pb.AccountPlanProto{Key: plan.Key, Label: plan.Label, CredentialKey: plan.CredentialKey, MatchMode: string(plan.MatchMode), Matches: slices.Clone(plan.Matches)})
	}
	return result
}

func accountPlansFromProto(plans []*pb.AccountPlanProto) []sdk.AccountPlan {
	if plans == nil {
		return nil
	}
	result := make([]sdk.AccountPlan, 0, len(plans))
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		result = append(result, sdk.AccountPlan{Key: plan.Key, Label: plan.Label, CredentialKey: plan.CredentialKey, MatchMode: sdk.AccountPlanMatch(plan.MatchMode), Matches: slices.Clone(plan.Matches)})
	}
	return result
}
