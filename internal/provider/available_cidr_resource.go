package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/massdriver-cloud/cola/pkg/cidr"

	"github.com/massdriver-cloud/terraform-provider-utility/internal/planmodifiers"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &AvailableCidrResource{}
var _ resource.ResourceWithImportState = &AvailableCidrResource{}

func NewAvailableCidrResource() resource.Resource {
	return &AvailableCidrResource{}
}

// AvailableCidrResource defines the resource implementation.
type AvailableCidrResource struct{}

// AvailableCidrResourceModel describes the resource data model.
type AvailableCidrResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Keepers     types.Map    `tfsdk:"keepers"`
	ParentCidrs types.List   `tfsdk:"parent_cidrs"`
	UsedCidrs   types.List   `tfsdk:"used_cidrs"`
	Mask        types.Int64  `tfsdk:"mask"`
	Result      types.String `tfsdk:"result"`
}

func (r *AvailableCidrResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_available_cidr"
}

func (r *AvailableCidrResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Available CIDR resource",

		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:            true,
				MarkdownDescription: "CIDR Identifier",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
				Type: types.StringType,
			},
			"parent_cidrs": {
				Description: "A list of the CIDR range(s) from which to search for available CIDR ranges. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Validators: []tfsdk.AttributeValidator{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValuesAre(stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(?:\.(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])){3}(?:\/(?:[1-9]|[1-2][0-9]|3[0-2]))$`), "Must be valid CIDR notation")),
				},
				Required: true,
			},
			"used_cidrs": {
				Description: "CIDR ranges that are already used within the `parent_cidrs` which should be avoided to prevent overlaps and/or collisions. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Validators: []tfsdk.AttributeValidator{
					listvalidator.ValuesAre(stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(?:\.(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])){3}(?:\/(?:[1-9]|[1-2][0-9]|3[0-2]))$`), "Must be valid CIDR notation")),
				},
				Required: true,
			},
			"mask": {
				Description: "Desired mask (network/subnet size) to find that is available. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
				Type:        types.Int64Type,
				Required:    true,
			},
			"keepers": {
				Description: "Arbitrary map of values that, when changed, will trigger recreation of resource. See [the main provider documentation](../index.html) for more information.",
				Type: types.MapType{
					ElemType: types.StringType,
				},
				Optional: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					planmodifiers.RequiresReplaceIfValuesNotNull(),
				},
			},
			"result": {
				Description: "The available CIDR that was found.",
				Computed:    true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
				Type: types.StringType,
			},
		},
	}, nil
}

func (r *AvailableCidrResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}
}

func (r *AvailableCidrResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AvailableCidrResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	mask := net.CIDRMask(int(data.Mask.ValueInt64()), 32)

	parentCidrsStrings := make([]string, len(data.ParentCidrs.Elements()))
	usedCidrsStrings := make([]string, len(data.UsedCidrs.Elements()))

	resp.Diagnostics.Append(data.ParentCidrs.ElementsAs(ctx, &parentCidrsStrings, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(data.UsedCidrs.ElementsAs(ctx, &usedCidrsStrings, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	usedCidrs := make([]*net.IPNet, len(usedCidrsStrings))
	for i, used := range usedCidrsStrings {
		_, usedCidr, parseErr := net.ParseCIDR(used)
		if parseErr != nil {
			resp.Diagnostics.AddError(
				"Error parsing used_cidrs",
				fmt.Sprintf("... details ... %s", parseErr.Error()),
			)
			return
		}
		usedCidrs[i] = usedCidr
	}

	var result *net.IPNet
	var findErr error
	for _, parent := range parentCidrsStrings {
		_, parentCidr, parseErr := net.ParseCIDR(parent)
		if parseErr != nil {
			resp.Diagnostics.AddError(
				"Error parsing parent_cidrs",
				fmt.Sprintf("... details ... %s", parseErr.Error()),
			)
			return
		}

		// The FindAvailableCIDR function errors if one of the "used" CIDRs
		// isn't contained in the parent. Since we can have multiple parents,
		// we should only pass the used CIDRs that are within the parent
		var containedCidrs []*net.IPNet
		for _, used := range usedCidrs {
			if cidr.ContainsCIDR(parentCidr, used) {
				containedCidrs = append(containedCidrs, used)
			}
		}

		result, findErr = cidr.FindAvailableCIDR(parentCidr, &mask, containedCidrs)
		if findErr != nil && !errors.Is(findErr, cidr.ErrNoAvailableCIDR) {
			resp.Diagnostics.AddError(
				"Error while finding available CIDR",
				fmt.Sprintf("... details ... %s", findErr.Error()),
			)
			return
		}
	}

	if findErr != nil {
		resp.Diagnostics.AddError(
			"No available CIDR found",
			fmt.Sprintf("... details ... %s", findErr.Error()),
		)
		return
	}

	data.Id = types.StringValue(result.String())
	data.Result = types.StringValue(result.String())

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "found an available cidr: "+result.String())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *AvailableCidrResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *AvailableCidrResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AvailableCidrResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *AvailableCidrResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *AvailableCidrResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
