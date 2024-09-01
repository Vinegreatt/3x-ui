package sub

import (
    "encoding/base64"
    "net"
    "strings"

    "github.com/gin-gonic/gin"
)

type SUBController struct {
    subPath        string
    subJsonPath    string
    subEncrypt     bool
    updateInterval string

    subService     *SubService
    subJsonService *SubJsonService
}

func NewSUBController(
    g *gin.RouterGroup,
    subPath string,
    jsonPath string,
    encrypt bool,
    showInfo bool,
    rModel string,
    update string,
    jsonFragment string,
    jsonNoise string,
    jsonMux string,
    jsonRules string,
) *SUBController {
    sub := NewSubService(showInfo, rModel)
    a := &SUBController{
        subPath:        subPath,
        subJsonPath:    jsonPath,
        subEncrypt:     encrypt,
        updateInterval: update,

        subService:     sub,
        subJsonService: NewSubJsonService(jsonFragment, jsonNoise, jsonMux, jsonRules, sub),
    }
    a.initRouter(g)
    return a
}

func (a *SUBController) initRouter(g *gin.RouterGroup) {
    gLink := g.Group(a.subPath)
    gJson := g.Group(a.subJsonPath)

    gLink.GET(":subid", a.subs)
    gJson.GET(":subid", a.subJsons)
}

func (a *SUBController) subs(c *gin.Context) {
    subId := c.Param("subid")
    host := resolveHost(c)

    subs, header, err := a.subService.GetSubs(subId, host)
    if err != nil || len(subs) == 0 {
        c.String(400, "Error retrieving subscriptions!")
    } else {
        result := buildSubsResponse(subs, host)

        // Add headers
        c.Writer.Header().Set("Subscription-Userinfo", header)
        c.Writer.Header().Set("Profile-Update-Interval", a.updateInterval)
        c.Writer.Header().Set("Profile-Title", subId)

        if a.subEncrypt {
            c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
        } else {
            c.String(200, result)
        }
    }
}

func (a *SUBController) subJsons(c *gin.Context) {
    subId := c.Param("subid")
    host := resolveHost(c)

    jsonSub, header, err := a.subJsonService.GetJson(subId, host)
    if err != nil || len(jsonSub) == 0 {
        c.String(400, "Error retrieving JSON data!")
    } else {

        // Add headers
        c.Writer.Header().Set("Subscription-Userinfo", header)
        c.Writer.Header().Set("Profile-Update-Interval", a.updateInterval)
        c.Writer.Header().Set("Profile-Title", subId)

        c.String(200, jsonSub)
    }
}

func resolveHost(c *gin.Context) string {
    if h, err := getHostFromXFH(c.GetHeader("X-Forwarded-Host")); err == nil {
        return h
    }
    if host := c.GetHeader("X-Real-IP"); host != "" {
        return host
    }
    if host, _, err := net.SplitHostPort(c.Request.Host); err == nil {
        return host
    }
    return c.Request.Host
}

func buildSubsResponse(subs []string, originalHost string) string {
    modifiedHost := "germ.realityvpn.ru"
    result := ""
    for _, sub := range subs {
        // Replace the host part of the original URL with the modified host
        modifiedSub := strings.Replace(sub, originalHost, modifiedHost, 1)
        result += sub + "\n" + modifiedSub + "\n"
    }
    return result
}

func getHostFromXFH(s string) (string, error) {
    if strings.Contains(s, ":") {
        realHost, _, err := net.SplitHostPort(s)
        if err != nil {
            return "", err
        }
        return realHost, nil
    }
    return s, nil
}
