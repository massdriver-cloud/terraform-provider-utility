package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/massdriver-cloud/cola/pkg/cidr"

	"github.com/massdriver-cloud/terraform-provider-utility/internal/planmodifiers"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	Id        types.String `tfsdk:"id"`
	Keepers   types.Map    `tfsdk:"keepers"`
	FromCidrs types.List   `tfsdk:"from_cidrs"`
	UsedCidrs types.List   `tfsdk:"used_cidrs"`
	Mask      types.Int64  `tfsdk:"mask"`
	Result    types.String `tfsdk:"result"`
}

func (r *AvailableCidrResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_available_cidr"
}

func (r *AvailableCidrResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Given CIDR range(s) to search over (ex. a Network) and a list of already used CIDR ranges (ex. a list of subnets) " +
			"find an unused, non-conflicting CIDR range of specified size.",

		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:            true,
				MarkdownDescription: "CIDR Identifier. The value will be identical to the `result` field.",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
				Type: types.StringType,
			},
			"from_cidrs": {
				MarkdownDescription: "A list containing the CIDR range(s) from which to search for available CIDR ranges. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
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
				MarkdownDescription: "A list containing the CIDR ranges that are already used within the `from_cidrs` block(s) which should be avoided to prevent overlaps and/or collisions. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Validators: []tfsdk.AttributeValidator{
					listvalidator.ValuesAre(stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(?:\.(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])){3}(?:\/(?:[1-9]|[1-2][0-9]|3[0-2]))$`), "Must be valid CIDR notation")),
				},
				Required: true,
			},
			"mask": {
				MarkdownDescription: "Desired mask (network/subnet size) to find that is available. Changing this value after creation **HAS NO EFFECT**. This allows the `result` CIDR to remain stable when it is used to find a range to create a network/subnet. If you would like to conditionally update this resource, use the `keepers` field.",
				Type:                types.Int64Type,
				Required:            true,
			},
			"keepers": {
				MarkdownDescription: "Arbitrary map of values that, when changed, will trigger re-creation of resource. This field works the same as the `keepers` field in the [`Random` provider](https://registry.terraform.io/providers/hashicorp/random/latest/docs#resource-keepers).",
				Type: types.MapType{
					ElemType: types.StringType,
				},
				Optional: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					planmodifiers.RequiresReplaceIfValuesNotNull(),
				},
			},
			"result": {
				MarkdownDescription: "The available CIDR that was found.",
				Computed:            true,
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

	fromCidrsStrings := make([]string, len(data.FromCidrs.Elements()))
	usedCidrsStrings := make([]string, len(data.UsedCidrs.Elements()))

	resp.Diagnostics.Append(data.FromCidrs.ElementsAs(ctx, &fromCidrsStrings, false)...)
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
	for _, from := range fromCidrsStrings {
		_, fromCidr, parseErr := net.ParseCIDR(from)
		if parseErr != nil {
			resp.Diagnostics.AddError(
				"Error parsing from_cidrs",
				fmt.Sprintf("... details ... %s", parseErr.Error()),
			)
			return
		}

		result, findErr = cidr.FindAvailableCIDR(fromCidr, &mask, usedCidrs)
		if result != nil {
			break
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
	validation := regexp.MustCompile(`^(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(?:\.(?:[0-9]|[0-9]{2}|1[0-9][0-9]|2[0-4][0-9]|25[0-5])){3}(?:\/(?:[1-9]|[1-2][0-9]|3[0-2]))$`)
	if !validation.Match([]byte(req.ID)) {
		resp.Diagnostics.AddError(
			"Malformed resource ID (CIDR)",
			"The ID that was given must be a valid CIDR range",
		)
		return
	}

	mask, err := strconv.Atoi(strings.Split(req.ID, "/")[1])
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing resource ID",
			fmt.Sprintf("Unable to extract mask from CIDR: %s", err.Error()),
		)
		return
	}

	state := AvailableCidrResourceModel{
		FromCidrs: types.ListNull(types.StringType),
		UsedCidrs: types.ListNull(types.StringType),
		Keepers:   types.MapNull(types.StringType),
		Mask:      types.Int64Value(int64(mask)),
		Id:        types.StringValue(req.ID),
		Result:    types.StringValue(req.ID),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
