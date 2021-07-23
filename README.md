## AsyncEvent
rpg,slg类游戏多为状态服务，会比较频繁的操作内存数据，如果将客户端及服务器间消息与定时器回调到统一到一个goroutine中处理，能比较方便的操作逻辑，逻辑处理不需要加锁

### 常用接口
* `CallEvent` 同步获取事件处理结果
* `SendEvent` 异步发送事件，不等待处理完成
* `Exec` 异步执行一个无参函数
* `SetTimeout` 一次触发的定时器
* `SetInterval` 循环周期触发的定时器

### 使用
* 创建事件管理器`event := aevent.NewEventManager()`
* 定义事件名及结构
```
const (
	asEventOnClientMsg = "asEventOnClientMsg"
)

type eventOnClientMessage struct {
	roleId  uint64
	cmd     protocol.EProtocol_Proto
	content []byte
}
```
* 注册事件处理
```
event.RegisterEvent(asEventOnClientMsg, func(event *aevent.Event) (interface{}, error) {
	return implEventOnClientMsg(event.Data().(*eventOnClientMessage))
})
```
* 开始处理事件消息 `event.Serve(ctx)`
* 触发事件
```
event.SendEvent(asEventOnClientMsg, &eventOnClientMessage{
			roleId:  msg.RoleId,
			cmd:     protocol.EProtocol_Proto(msg.MsgId),
			content: msg.MsgContent,
		})
```  
