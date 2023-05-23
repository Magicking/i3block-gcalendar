# i3block-gcalender

## How to get token?

 -> Create https://console.cloud.google.com/apis/credentials?project=YOUR_PROJECT_NAME
  - Create OAuth client ID
  - Desktop App
-> Save credentials to ~/.config/i3block-gcalendar/credentials.json

## How to add a watched calendar ?

1$> ~/.config/i3blocks/calendar register
2 Following URL
3 Choose account
4 Copy paste the value of the code part (look like this 4 4/0AbURXXXXXXXXXXXXXXXXXVGuO-fwOCA)in the console
It will register the access token with read only scope in ~/.config/i3block-gcalendar/auth-tokens/

## How to add to i3blocks ?

Add to your ~/.i3/config

```ini
[calendar]
command=/home/magicking/.config/i3blocks/calendar
interval=60
markup=pango
```

## Config

List of register token calendar
%> cat ~/.config/i3block-gcalendar/config.yml
```yaml
auth-tokens:
    - /home/magicking/.config/i3block-gcalendar/auth-tokens/2cf3e8a1-b838-499b-bbfa-0556f126911d
    - /home/magicking/.config/i3block-gcalendar/auth-tokens/07b96584-8b1e-41ce-8dde-11da94455d89

```