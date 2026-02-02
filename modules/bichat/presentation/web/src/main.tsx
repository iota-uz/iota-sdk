import { registerChatRootElement } from './ChatRootElement'
import { injectMockContext } from './dev/mockIotaContext'

injectMockContext()
registerChatRootElement()
