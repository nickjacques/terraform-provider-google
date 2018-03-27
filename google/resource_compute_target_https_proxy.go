package google

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/terraform/helper/schema"
	"google.golang.org/api/compute/v1"
)

const (
	canonicalSslCertificateTemplate = "https://www.googleapis.com/compute/v1/projects/%s/global/sslCertificates/%s"
)

func resourceComputeTargetHttpsProxy() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeTargetHttpsProxyCreate,
		Read:   resourceComputeTargetHttpsProxyRead,
		Delete: resourceComputeTargetHttpsProxyDelete,
		Update: resourceComputeTargetHttpsProxyUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"ssl_certificates": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					DiffSuppressFunc: compareSelfLinkOrResourceName,
				},
			},

			"url_map": &schema.Schema{
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: compareSelfLinkRelativePaths,
			},

			"ssl_policy": &schema.Schema{
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: compareSelfLinkRelativePaths,
			},

			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"self_link": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"proxy_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
		},
	}
}

func resourceComputeTargetHttpsProxyCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	sslCertificates, err := expandSslCertificates(d, config)
	if err != nil {
		return err
	}

	proxy := &compute.TargetHttpsProxy{
		Name:            d.Get("name").(string),
		UrlMap:          d.Get("url_map").(string),
		SslCertificates: sslCertificates,
	}

	if v, ok := d.GetOk("description"); ok {
		proxy.Description = v.(string)
	}

	log.Printf("[DEBUG] TargetHttpsProxy insert request: %#v", proxy)
	op, err := config.clientCompute.TargetHttpsProxies.Insert(
		project, proxy).Do()
	if err != nil {
		return fmt.Errorf("Error creating TargetHttpsProxy: %s", err)
	}

	err = computeOperationWait(config.clientCompute, op, project, "Creating Target Https Proxy")
	if err != nil {
		return err
	}

	d.SetId(proxy.Name)

	if v, ok := d.GetOk("ssl_policy"); ok {
		pol, err := ParseSslPolicyFieldValue(v.(string), d, config)
		op, err := config.clientCompute.TargetHttpsProxies.SetSslPolicy(
			project, proxy.Name, &compute.SslPolicyReference{
				SslPolicy: pol.RelativeLink(),
			}).Do()
		if err != nil {
			return errwrap.Wrapf("Error setting Target HTTPS Proxy SSL Policy: {{err}}", err)
		}
		waitErr := computeSharedOperationWait(config.clientCompute, op, project, "Adding Target HTTPS Proxy SSL Policy")
		if waitErr != nil {
			return waitErr
		}
	}

	return resourceComputeTargetHttpsProxyRead(d, meta)
}

func resourceComputeTargetHttpsProxyUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	d.Partial(true)

	if d.HasChange("url_map") {
		url_map := d.Get("url_map").(string)
		url_map_ref := &compute.UrlMapReference{UrlMap: url_map}
		op, err := config.clientCompute.TargetHttpsProxies.SetUrlMap(
			project, d.Id(), url_map_ref).Do()
		if err != nil {
			return fmt.Errorf("Error updating Target HTTPS proxy URL map: %s", err)
		}

		err = computeOperationWait(config.clientCompute, op, project, "Updating Target Https Proxy URL Map")
		if err != nil {
			return err
		}

		d.SetPartial("url_map")
	}

	if d.HasChange("ssl_certificates") {
		certs, err := expandSslCertificates(d, config)
		if err != nil {
			return err
		}
		cert_ref := &compute.TargetHttpsProxiesSetSslCertificatesRequest{
			SslCertificates: certs,
		}
		op, err := config.clientCompute.TargetHttpsProxies.SetSslCertificates(
			project, d.Id(), cert_ref).Do()
		if err != nil {
			return fmt.Errorf("Error updating Target Https Proxy SSL Certificates: %s", err)
		}

		err = computeOperationWait(config.clientCompute, op, project, "Updating Target Https Proxy SSL certificates")
		if err != nil {
			return err
		}

		d.SetPartial("ssl_certificate")
	}

	if d.HasChange("ssl_policy") {
		pol, err := ParseSslPolicyFieldValue(d.Get("ssl_policy").(string), d, config)
		if err != nil {
			return err
		}
		op, err := config.clientCompute.TargetHttpsProxies.SetSslPolicy(
			project, d.Id(), &compute.SslPolicyReference{
				SslPolicy: pol.RelativeLink(),
			}).Do()
		if err != nil {
			return err
		}
		waitErr := computeSharedOperationWait(config.clientCompute, op, project, "Updating Target HTTPS Proxy SSL Policy")
		if waitErr != nil {
			return waitErr
		}
	}

	d.Partial(false)

	return resourceComputeTargetHttpsProxyRead(d, meta)
}

func resourceComputeTargetHttpsProxyRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	proxy, err := config.clientCompute.TargetHttpsProxies.Get(
		project, d.Id()).Do()
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Target HTTPS proxy %q", d.Get("name").(string)))
	}

	d.Set("ssl_certificates", proxy.SslCertificates)
	d.Set("proxy_id", strconv.FormatUint(proxy.Id, 10))
	d.Set("self_link", proxy.SelfLink)
	d.Set("description", proxy.Description)
	d.Set("url_map", proxy.UrlMap)
	d.Set("name", proxy.Name)
	d.Set("project", project)
	d.Set("ssl_policy", proxy.SslPolicy)

	return nil
}

func resourceComputeTargetHttpsProxyDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	// Delete the TargetHttpsProxy
	log.Printf("[DEBUG] TargetHttpsProxy delete request")
	op, err := config.clientCompute.TargetHttpsProxies.Delete(
		project, d.Id()).Do()
	if err != nil {
		return fmt.Errorf("Error deleting TargetHttpsProxy: %s", err)
	}

	err = computeOperationWait(config.clientCompute, op, project, "Deleting Target Https Proxy")
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}

func expandSslCertificates(d *schema.ResourceData, config *Config) ([]string, error) {
	configured := d.Get("ssl_certificates").([]interface{})
	certs := make([]string, 0, len(configured))

	for _, sslCertificate := range configured {
		sslCertificateFieldValue, err := ParseSslCertificateFieldValue(sslCertificate.(string), d, config)
		if err != nil {
			return nil, fmt.Errorf("Invalid ssl certificate: %s", err)
		}

		certs = append(certs, sslCertificateFieldValue.RelativeLink())
	}

	return certs, nil
}
