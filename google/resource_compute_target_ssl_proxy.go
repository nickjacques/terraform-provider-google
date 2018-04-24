package google

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/terraform/helper/schema"
	"google.golang.org/api/compute/v1"
)

func resourceComputeTargetSslProxy() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeTargetSslProxyCreate,
		Read:   resourceComputeTargetSslProxyRead,
		Delete: resourceComputeTargetSslProxyDelete,
		Update: resourceComputeTargetSslProxyUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"backend_service": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"ssl_certificates": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					DiffSuppressFunc: compareSelfLinkOrResourceName,
				},
			},

			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"proxy_header": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "NONE",
			},

			"ssl_policy": &schema.Schema{
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: compareSelfLinkRelativePaths,
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"proxy_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"self_link": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceComputeTargetSslProxyCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	sslCertificates, err := expandSslCertificates(d, config)
	if err != nil {
		return err
	}

	proxy := &compute.TargetSslProxy{
		Name:            d.Get("name").(string),
		Service:         d.Get("backend_service").(string),
		ProxyHeader:     d.Get("proxy_header").(string),
		Description:     d.Get("description").(string),
		SslCertificates: sslCertificates,
	}

	log.Printf("[DEBUG] TargetSslProxy insert request: %#v", proxy)
	op, err := config.clientCompute.TargetSslProxies.Insert(
		project, proxy).Do()
	if err != nil {
		return fmt.Errorf("Error creating TargetSslProxy: %s", err)
	}

	err = computeOperationWait(config.clientCompute, op, project, "Creating Target Ssl Proxy")
	if err != nil {
		return err
	}

	d.SetId(proxy.Name)

	if v, ok := d.GetOk("ssl_policy"); ok {
		pol, err := ParseSslPolicyFieldValue(v.(string), d, config)
		op, err := config.clientCompute.TargetSslProxies.SetSslPolicy(
			project, proxy.Name, &compute.SslPolicyReference{
				SslPolicy: pol.RelativeLink(),
			}).Do()
		if err != nil {
			return errwrap.Wrapf("Error setting SSL Policy: {{err}}", err)
		}
		waitErr := computeSharedOperationWait(config.clientCompute, op, project, "Adding Target SSL Proxy SSL Policy")
		if waitErr != nil {
			return waitErr
		}
	}

	return resourceComputeTargetSslProxyRead(d, meta)
}

func resourceComputeTargetSslProxyUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	d.Partial(true)

	if d.HasChange("proxy_header") {
		proxyHeader := d.Get("proxy_header").(string)
		proxyHeaderPayload := &compute.TargetSslProxiesSetProxyHeaderRequest{
			ProxyHeader: proxyHeader,
		}
		op, err := config.clientCompute.TargetSslProxies.SetProxyHeader(
			project, d.Id(), proxyHeaderPayload).Do()
		if err != nil {
			return fmt.Errorf("Error updating proxy_header: %s", err)
		}

		err = computeOperationWait(config.clientCompute, op, project, "Updating Target SSL Proxy")
		if err != nil {
			return err
		}

		d.SetPartial("proxy_header")
	}

	if d.HasChange("backend_service") {
		op, err := config.clientCompute.TargetSslProxies.SetBackendService(project, d.Id(), &compute.TargetSslProxiesSetBackendServiceRequest{
			Service: d.Get("backend_service").(string),
		}).Do()

		if err != nil {
			return fmt.Errorf("Error updating backend_service: %s", err)
		}

		err = computeOperationWait(config.clientCompute, op, project, "Updating Target SSL Proxy")
		if err != nil {
			return err
		}

		d.SetPartial("backend_service")
	}

	if d.HasChange("ssl_certificates") {
		sslCertificates, err := expandSslCertificates(d, config)
		if err != nil {
			return err
		}

		op, err := config.clientCompute.TargetSslProxies.SetSslCertificates(project, d.Id(), &compute.TargetSslProxiesSetSslCertificatesRequest{
			SslCertificates: sslCertificates,
		}).Do()

		if err != nil {
			return fmt.Errorf("Error updating backend_service: %s", err)
		}

		err = computeOperationWait(config.clientCompute, op, project, "Updating Target SSL Proxy")
		if err != nil {
			return err
		}

		d.SetPartial("ssl_certificates")
	}

	if d.HasChange("ssl_policy") {
		pol, err := ParseSslPolicyFieldValue(d.Get("ssl_policy").(string), d, config)
		if err != nil {
			return err
		}
		op, err := config.clientCompute.TargetSslProxies.SetSslPolicy(
			project, d.Id(), &compute.SslPolicyReference{
				SslPolicy: pol.RelativeLink(),
			}).Do()
		if err != nil {
			return err
		}
		waitErr := computeSharedOperationWait(config.clientCompute, op, project, "Updating Target SSL Proxy SSL Policy")
		if waitErr != nil {
			return waitErr
		}
	}

	d.Partial(false)

	return resourceComputeTargetSslProxyRead(d, meta)
}

func resourceComputeTargetSslProxyRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	proxy, err := config.clientCompute.TargetSslProxies.Get(
		project, d.Id()).Do()
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Target SSL Proxy %q", d.Get("name").(string)))
	}

	d.Set("name", proxy.Name)
	d.Set("description", proxy.Description)
	d.Set("proxy_header", proxy.ProxyHeader)
	d.Set("backend_service", proxy.Service)
	d.Set("ssl_certificates", proxy.SslCertificates)
	d.Set("ssl_policy", proxy.SslPolicy)
	d.Set("project", project)
	d.Set("self_link", proxy.SelfLink)
	d.Set("proxy_id", strconv.FormatUint(proxy.Id, 10))

	return nil
}

func resourceComputeTargetSslProxyDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	op, err := config.clientCompute.TargetSslProxies.Delete(
		project, d.Id()).Do()
	if err != nil {
		return fmt.Errorf("Error deleting TargetSslProxy: %s", err)
	}

	err = computeOperationWait(config.clientCompute, op, project, "Deleting Target SSL Proxy")
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
