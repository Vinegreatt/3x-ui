package sub

import (
    "encoding/base64"
    "fmt"
    "net/url"
    "strings"
    "time"

    "x-ui/database"
    "x-ui/database/model"
    "x-ui/logger"
    "x-ui/util/common"
    "x-ui/util/random"
    "x-ui/web/service"
    "x-ui/xray"

    "github.com/goccy/go-json"
)

type SubService struct {
    address        string
    showInfo       bool
    remarkModel    string
    datepicker     string
    inboundService service.InboundService
    settingService service.SettingService
}

func NewSubService(showInfo bool, remarkModel string) *SubService {
    return &SubService{
        showInfo:    showInfo,
        remarkModel: remarkModel,
    }
}

func (s *SubService) GetSubs(subId string, host string) ([]string, string, error) {
    s.address = host
    var result []string
    var header string
    var traffic xray.ClientTraffic
    var clientTraffics []xray.ClientTraffic
    inbounds, err := s.getInboundsBySubId(subId)
    if err != nil {
        return nil, "", err
    }

    if len(inbounds) == 0 {
        return nil, "", common.NewError("No inbounds found with ", subId)
    }

    s.datepicker, err = s.settingService.GetDatepicker()
    if err != nil {
        s.datepicker = "gregorian"
    }
    for _, inbound := range inbounds {
        clients, err := s.inboundService.GetClients(inbound)
        if err != nil {
            logger.Error("SubService - GetClients: Unable to get clients from inbound")
        }
        if clients == nil {
            continue
        }
        if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
            listen, port, streamSettings, err := s.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
            if err == nil {
                inbound.Listen = listen
                inbound.Port = port
                inbound.StreamSettings = streamSettings
            }
        }
        for _, client := range clients {
            if client.Enable && client.SubID == subId {
                originalLink := s.getLink(inbound, client.Email)
                result = append(result, originalLink) // Добавляем оригинальную ссылку

                modifiedLink := s.getLinkWithModifiedHost(inbound, client.Email, "germ.realityvpn.ru")
                result = append(result, modifiedLink) // Добавляем модифицированную ссылку
                clientTraffics = append(clientTraffics, s.getClientTraffics(inbound.ClientStats, client.Email))
            }
        }
    }

    // Prepare statistics
    for index, clientTraffic := range clientTraffics {
        if index == 0 {
            traffic.Up = clientTraffic.Up
            traffic.Down = clientTraffic.Down
            traffic.Total = clientTraffic.Total
            if clientTraffic.ExpiryTime > 0 {
                traffic.ExpiryTime = clientTraffic.ExpiryTime
            }
        } else {
            traffic.Up += clientTraffic.Up
            traffic.Down += clientTraffic.Down
            if traffic.Total == 0 || clientTraffic.Total == 0 {
                traffic.Total = 0
            } else {
                traffic.Total += clientTraffic.Total
            }
            if clientTraffic.ExpiryTime != traffic.ExpiryTime {
                traffic.ExpiryTime = 0
            }
        }
    }
    header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
    return result, header, nil
}

func (s *SubService) getInboundsBySubId(subId string) ([]*model.Inbound, error) {
    db := database.GetDB()
    var inbounds []*model.Inbound
    err := db.Model(model.Inbound{}).Preload("ClientStats").Where(`id in (
        SELECT DISTINCT inbounds.id
        FROM inbounds,
            JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client 
        WHERE
            protocol in ('vmess','vless','trojan','shadowsocks')
            AND JSON_EXTRACT(client.value, '$.subId') = ? AND enable = ?
    )`, subId, true).Find(&inbounds).Error
    if err != nil {
        return nil, err
    }
    return inbounds, nil
}

func (s *SubService) getClientTraffics(traffics []xray.ClientTraffic, email string) xray.ClientTraffic {
    for _, traffic := range traffics {
        if traffic.Email == email {
            return traffic
        }
    }
    return xray.ClientTraffic{}
}

func (s *SubService) getLinkWithModifiedHost(inbound *model.Inbound, email string, newHost string) string {
    link := s.getLink(inbound, email)
    url, err := url.Parse(link)
    if err
