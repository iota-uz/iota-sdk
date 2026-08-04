import { describe, expect, it } from 'vitest'
import { biChatHTTPConfigFromRoute, type StandardAppletInitial } from './applet'
import type { ClientRouteContext } from './route'

describe('biChatHTTPConfigFromRoute', () => {
  it('keeps route-ready RPC and stream endpoints independent of the applet mount path', () => {
    const route: ClientRouteContext<StandardAppletInitial> = {
      protocolVersion: '1.0.0',
      theme: 'light',
      csrf: 'csrf-token',
      initial: {
        appletContext: {
          config: {
            basePath: '/admin/ali/chat',
            rpcUIEndpoint: '/rpc',
            streamEndpoint: '/admin/ali/chat/stream',
          },
        },
      },
    }

    expect(biChatHTTPConfigFromRoute(route)).toMatchObject({
      baseUrl: '',
      rpcEndpoint: '/rpc',
      streamEndpoint: '/admin/ali/chat/stream',
      csrfToken: 'csrf-token',
    })
  })
})
