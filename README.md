## 踩坑记录
### Windows下开发流程
现在有go环境和kubectl能访问到apiserver的Linux机器上或者在docker容器里（需要挂载进去kubectl和k8s配置文件以及一个生成代码的目录）执行如下命令：
```shell
mkdir podsbook
cd podsbook
#初始化一个crd的项目
kubebuilder init --domain newcrd.com --repo github.com/newcrd
#创建一个自定义资源 交互中两个选项都输入y，一个是创建cr的yaml模板，一个是生成自定义控制器的代码
kubebuilder create api --group apps --version v1beta1 --kind NewDep
```
注意版本号，版本号的规则是：`Version must match ^v\d+(?:alpha\d+|beta\d+)?$`
打包代码到本地，用编辑器打开，开始写自己的代码
#### 定义自己的crd的字段
[newdep_types.go](api%2Fv1beta1%2Fnewdep_types.go)
```go
package v1beta1
// NewDepSpec defines the desired state of NewDep
type NewDepSpec struct {
	// Image image name <name:tag>
	Image *string `json:"image,omitempty"`
	// Replica number of pods
	Replica *int32 `json:"replica,omitempty"`
}

// NewDepStatus defines the observed state of NewDep
type NewDepStatus struct {
	// RealReplica number of pods
	RealReplica int32 `json:"realReplica,omitempty"`
}
```
spec是crd的字段，status是crd的状态值，是一个子资源
```go
package v1beta1
//37行
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.realReplica",name=RealReplica,type=integer
```
这三行注解，前两个是自动生成的，最后一个是自己添加的，用于在执行kubectl get newdeps.apps.newcrd.com 命令的时候添加一列状态字段。
体现在控制台是这样的
```shell
root@master-1:~# kubectl get newdeps.apps.newcrd.com 
NAME            REALREPLICA
newdep-sample   3
```
接下来提交代码到git仓库，然后在Linux机器或者docker容器里把代码拉下来，执行make命令，补全cr、crd的yaml文件
```shell
make manifests generate
```
crd的文件在[apps.newcrd.com_newdeps.yaml](config%2Fcrd%2Fbases%2Fapps.newcrd.com_newdeps.yaml)
上边手动添加打印字段注解生成的配置体现在这里
```yaml
  - additionalPrinterColumns:
    - jsonPath: .status.realReplica
      name: RealReplica
      type: integer
```
部署cr的yaml文件在[apps_v1beta1_newdep.yaml](config%2Fsamples%2Fapps_v1beta1_newdep.yaml)，image字段和replica字段需要自己填写。
执行完命令后提交代码，然后在本地把代码拉下来，然后开始补全自定义控制器的逻辑，demo里的逻辑是创建newpods对象后，自动创建一个同名的deployment，更新newpods对象后，对应deployment的副本数也会变化。
#### 实现自定义控制器业务逻辑
主要代码在[newdep_controller.go](internal%2Fcontroller%2Fnewdep_controller.go)文件，有几个踩坑点
##### 更新crd的状态的时候，不能直接调用Update方法，因为它是子资源，无法通过更新主资源来进行更新
```go
newcrd.Status.RealReplica = *newcrd.Spec.Replica
err = r.Status().Update(ctx, newcrd)
if err != nil {
    r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
    return ctrl.Result{}, err
}
```
#### 因为要操作deployment，所以还需要添加一个rbac的注解，给serveraccount添加deployment相关权限
```go
// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps/finalizers,verbs=update
// 这一行
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
```
##### 关联crd资源和deployment，删除crd的时候同时删除deployment，这里要更新一下deployment对象才行，SetControllerReference方法只是往deployment对象里添加的关联属性，并未更新etcd数据
```go
deploy := createDeployment(newcrd)
if err = ctrl.SetControllerReference(newcrd, deploy, r.Scheme); err != nil {
    return ctrl.Result{}, err
}
err = r.Create(ctx, deploy)
```
同时还要更新SetupWithManager方法
```go
func (r *NewDepReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1beta1.NewDep{}).
		//加了这一行
		Owns(&k8sappsv1.Deployment{}).
		Complete(r)
}
```
##### 写event事件
首先给自定义控制器结构体添加一个字段
```go
type NewDepReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	//这里
	Recorder record.EventRecorder
}
```
然后main函数里也要加入相关逻辑[main.go](cmd%2Fmain.go)
```go
if err = (&controller.NewDepReconciler{
    Client:   mgr.GetClient(),
    Scheme:   mgr.GetScheme(),
	//这里
    Recorder: mgr.GetEventRecorderFor("Newcrd"),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "NewDep")
    os.Exit(1)
}
```
使用方法
```go
newcrd.Status.RealReplica = *newcrd.Spec.Replica
err = r.Status().Update(ctx, newcrd)
if err != nil {
    r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
    return ctrl.Result{}, err
}
```
#### 统一安装依赖包和部署相关资源
自定义控制器代码写完了以后，提交代码，然后在Linux机器或者docker容器里把代码拉下来，执行`make install`命令。
执行完之后就可以在本地运行代码了，前提是本地要再家目录的.kube目录下有k8s的配置文件，这样才能访问到apiserver。
当我们apply一个cr文件后，就会触发自定义控制器的代码，就会创建deployment，我们更新了cr里的副本或者镜像名称的时候，deployment也会同时进行更新。
#### 完结 撒花