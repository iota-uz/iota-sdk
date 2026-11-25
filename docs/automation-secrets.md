# Секреты автоматизаций: встроенный коннектор «Secret»

Секреты вида `API_KEY` арендатор ведёт на странице «Интеграции» как встроенный коннектор «Secret».
Запись секрета (имя, описание, ссылка на значение, версии, времена создания, ротации и последнего
использования) лежит в `automation.secrets`; **значение** лежит за портом хранилища
(`pkg/automation/secrets`). Страница и ноды видят только запись; значение после сохранения не показывается.

## Порт хранилища

```go
type Writer interface {
    Put(ctx, ref Ref, data map[string]string) (Metadata, error) // следующая версия значения
    Delete(ctx, ref Ref) error                                  // со всеми версиями; нет значения - не ошибка
}
type Reader interface{ Read(ctx, ref Ref) (Value, error) }                // текущая версия целиком
type MetadataReader interface{ Metadata(ctx, ref Ref) (Metadata, error) } // версии и времена, не значение
type Store interface{ Writer; Reader; MetadataReader }
```

`Ref{TenantID, Path}` - где лежит значение. Путь формирует `Locator`; по умолчанию
`DefaultLocator(prefix)`: `<prefix>tenants/<tenant>/automation/secrets/<secret-id>` (prefix - окружение,
например `prod/`). Запись хранит `ref_path`, так что смена Locator не ломает уже созданные секреты.

Процесс может держать только `Writer` (управление) или только `Reader` (резолв в нодах): композиция
выдаёт каждому потребителю его часть.

### Контракт адаптера

- `Read` возвращает одну версию целиком, никогда поля двух версий (у встроенного адаптера - одна строка и один запрос).
- `Put` виден следующему `Read` в любом процессе; кеша, переживающего ротацию, нет. Версия растёт атомарно.
- `Ref` без арендатора - `ErrForbidden`; путь чужого арендатора - `ErrNotFound`/`ErrForbidden`, значение не отдаётся.
- Недоступность хранилища, неподходящий ключ, повреждённая строка - `ErrUnavailable`. Вызывающий **никогда** не
  подменяет её переменной окружения или устаревшим значением.
- Ошибки не содержат значений.

## Встроенный адаптер

`connectors.NewSealedSecrets(db, keys)` хранит значение в `automation.secret_values`
(`tenant_id, path, version, key_id, nonce, ciphertext`, PK `(tenant_id, path)`): AES-256-GCM тем же кольцом
ключей, что и подключения; AAD связывает шифртекст с арендатором, путём и версией. Без кольца ключей
каждый вызов - `ErrUnavailable`, оборачивающий `connectors.ErrNotConfigured`. `Store.Rewrap` пере-шифровывает
значения секретов вместе с подключениями.

## Менеджер и резолвер

- `secrets.NewManager(db, writer, meta, locator)`: `List`, `Create`, `Update` (имя, описание; с проверкой версии),
  `Rotate` (новая версия значения), `Delete` (значение и запись). Значение - не пустое, до 64 КиБ; имя -
  `^[A-Z][A-Z0-9_]{0,63}$`, уникально в арендаторе (`ErrNameTaken`). Значение пишется первым; если запись
  не вставилась - значение удаляется.
- `secrets.NewResolver(db, reader)`: `Resolve(ctx, tenant, name)`. Нет записи - `ErrNotFound`; сбой записей
  или хранилища - `ErrUnavailable`. Значение читается при каждом вызове, поэтому ротация действует в
  работающем процессе без перезапуска. `last_used_at` обновляется не чаще раза в минуту и не может
  провалить чтение.

## Порядок разрешения имени в ноде

Нода (Code, HTTP) объявляет секреты по имени. `access.ChainSecrets` ищет:

1. секрет арендатора (коннектор «Secret»). Субъект, от имени которого идёт запуск, должен иметь
   `Secrets.Use`; иначе нода падает с кодом `secret_forbidden`;
2. цепочка приложения (`access.FromResolver`): `CONNECTION__FIELD` - секретное поле подключения, затем `host.SecretResolver`;
3. иначе код `secret_missing`, сообщение `unknown secret NAME`.

`ErrUnavailable` хранилища арендатора - управляемая ошибка `unavailable` (повторяемая); к цепочке приложения
в этом случае **не** переходим.

## RBAC

- `Secrets.Manage` - создавать, переименовывать, ротировать, удалять (набор прав `secrets_manage`);
- `Secrets.Use` - автоматизации пользователя читают секреты арендатора (набор `secrets_use`).

## Заменить адаптер

```go
composition.Provide[secrets.Store](b, func(c *composition.Container) (secrets.Store, error) {
    return myopenbao.New(...), nil // Writer + Reader + MetadataReader
})
```

Менеджер, резолвер и Locator собираются поверх `secrets.Store`, записи остаются в PostgreSQL.

## Следующий шаг: адаптер OpenBao KV v2 (#1382)

| Порт | KV v2 (mount `secret`) |
| --- | --- |
| `Put(ref, data)` | `POST /v1/secret/data/<ref.Path>` с `{"data": {...}, "options": {"cas": <версия>}}`; версия из ответа - `Metadata.Version` |
| `Read(ref)` | `GET /v1/secret/data/<ref.Path>` - текущая версия целиком (`data.data`, `data.metadata.version`); 404 - `ErrNotFound` |
| `Metadata(ref)` | `GET /v1/secret/metadata/<ref.Path>` - `current_version`, `created_time`, `updated_time` |
| `Delete(ref)` | `DELETE /v1/secret/metadata/<ref.Path>` - все версии; 404 не ошибка |

Политики разделены: роль записи (`create`, `update` на `secret/data/tenants/*/automation/secrets/*`,
`delete` на `secret/metadata/...`) и роль чтения (`read` на `secret/data/...` и `secret/metadata/...`).
Сетевые сбои, 5xx, sealed/standby Vault - `ErrUnavailable`; 403 по токену процесса - тоже, не `ErrForbidden`
(последний - только для чужого арендатора). Префикс окружения задаётся `DefaultLocator("prod/")`.
