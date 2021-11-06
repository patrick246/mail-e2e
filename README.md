# Mail End-to-end monitoring

This tool simulates the end to end flow for mail handling, and can detect mail deliverability issues with your setup.
It delivers mail to your SMTP server, and checks that the mail can be received through IMAP. 

## Configuration
```yaml
metricsPort: 8080
targets:
  - name: your-target-name
    smtp:
      hostname: smtp.example.com
      port: 25
      #username: optional
      #password: optional
      from: mail-e2e@example.com
      to: mail-e2e-target@example.com
    imap:
      hostname: imap.example.com
      port: 993
      username: mail-e2e-target@example.com
      password: your-mailbox-password
      insecureSkipVerify: false
```
This file will be read from /etc/mail-e2e/config.yaml if it is present. You can configure this path by setting the 
`MAILE2E_CONFIG_FILE` environment variable.

## Exported Metrics
- **maile2e_smtp_mail_sent_total**: The total number of mails sent to the mailbox since the start of this program, including errors
- **maile2e_smtp_mail_sent_error_total**: The number of errors encountered while sending mails to the mailbox
- **maile2e_imap_mail_received_total**: The number of attempts to receive the mail back from the IMAP server, including errors
- **maile2e_imap_mail_received_error_total**: The number of errors encountered while trying to receive the mail back from the IMAP server
- **maile2e_delivery_delay**: Histogram that records the time between sending the initial mail, and getting the mail back from the IMAP server.
