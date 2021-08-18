package Cx

CxPolicy[result] {
	statefulset := input.document[i]
	statefulset.kind == "StatefulSet"

	count({x | resource := input.document[x]; resource.kind == "Service"; resource.spec.clusterIP == "None"; statefulset.metadata.namespace == resource.metadata.namespace; statefulset.spec.serviceName == resource.metadata.name; labelsMatch(statefulset, resource) == true}) == 0

	metadata := statefulset.metadata.name

	result := {
		"documentId": input.document[i].id,
		"searchKey": sprintf("metadata.name=%s.spec.serviceName", [metadata]),
		"issueType": "IncorrectValue",
		"keyExpectedValue": sprintf("metadata.name=%s.spec.serviceName refers to a Headless Service", [metadata]),
		"keyActualValue": sprintf("metadata.name=%s.spec.serviceName doesn't refers to a Headless Service", [metadata]),
	}
}

labelsMatch(stateful, service) {
	ser := {x | serv := service.spec.selector[n]; n != "_kics_lines"; x = serv}
	stat := {y | state := stateful.spec.template.metadata.labels[m]; m != "_kics_lines"; y = state}
	ser == stat
}
